package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"varsity-network/internal/database"
	"varsity-network/internal/models"
	"varsity-network/internal/routes"
	"varsity-network/pkg/utils"
)

// RequestWithdrawalHandler lets a teacher request a payout. The requested
// amount is deducted from their wallet balance immediately (so they can't
// request more than they have, and can't double-spend it while the request
// is pending) and held in the withdrawal_requests table until an admin
// manually sends the money (outside this system, e.g. via bKash Send Money)
// and marks the request "paid" — the same manual-review pattern as teacher
// KYC approval.
func RequestWithdrawalHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		claims, ok := r.Context().Value(routes.UserContextKey).(*utils.Claims)
		if !ok || claims.Role != "teacher" {
			http.Error(w, "Only teachers can request a withdrawal", http.StatusForbidden)
			return
		}

		var req models.WithdrawalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Amount <= 0 || req.PaymentNumber == "" {
			http.Error(w, "Invalid withdrawal request. Amount and payment_number are required.", http.StatusBadRequest)
			return
		}
		if req.Method == "" {
			req.Method = "bkash"
		}

		tx, err := database.DB.Begin()
		if err != nil {
			http.Error(w, "Transaction error", http.StatusInternalServerError)
			return
		}

		var currentBalance float64
		err = tx.QueryRow(`SELECT balance FROM users WHERE id = $1 FOR UPDATE`, claims.UserID).Scan(&currentBalance)
		if err != nil {
			tx.Rollback()
			http.Error(w, "User not found", http.StatusBadRequest)
			return
		}
		if currentBalance < req.Amount {
			tx.Rollback()
			http.Error(w, "Insufficient balance for this withdrawal amount", http.StatusBadRequest)
			return
		}

		_, err = tx.Exec(`UPDATE users SET balance = balance - $1 WHERE id = $2`, req.Amount, claims.UserID)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to deduct balance", http.StatusInternalServerError)
			return
		}

		var withdrawalID int
		err = tx.QueryRow(
			`INSERT INTO withdrawal_requests (teacher_id, amount, payment_number, method, status)
			 VALUES ($1, $2, $3, $4, 'pending') RETURNING id`,
			claims.UserID, req.Amount, req.PaymentNumber, req.Method,
		).Scan(&withdrawalID)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to create withdrawal request", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Commit failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        "success",
			"message":       "Withdrawal request submitted. An admin will review and pay it out manually.",
			"withdrawal_id": withdrawalID,
		})
	}
}

// GetMyWithdrawalsHandler lets a teacher see the status of their own requests.
func GetMyWithdrawalsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		claims, ok := r.Context().Value(routes.UserContextKey).(*utils.Claims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		rows, err := database.DB.Query(
			`SELECT id, teacher_id, amount, payment_number, method, status, COALESCE(admin_note, ''), requested_at, processed_at
			 FROM withdrawal_requests WHERE teacher_id = $1 ORDER BY requested_at DESC`,
			claims.UserID,
		)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		list := []models.WithdrawalResponse{}
		for rows.Next() {
			var wr models.WithdrawalResponse
			var processedAt sql.NullTime
			if err := rows.Scan(&wr.ID, &wr.TeacherID, &wr.Amount, &wr.PaymentNumber, &wr.Method, &wr.Status, &wr.AdminNote, &wr.RequestedAt, &processedAt); err != nil {
				continue
			}
			if processedAt.Valid {
				wr.ProcessedAt = &processedAt.Time
			}
			list = append(list, wr)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "data": list})
	}
}

// GetPendingWithdrawalsHandler (admin) lists withdrawal requests awaiting review.
func GetPendingWithdrawalsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		rows, err := database.DB.Query(`
			SELECT wr.id, wr.teacher_id, wr.amount, wr.payment_number, wr.method, wr.status,
				COALESCE(wr.admin_note, ''), wr.requested_at, wr.processed_at, u.name
			FROM withdrawal_requests wr
			JOIN users u ON u.id = wr.teacher_id
			WHERE wr.status = 'pending'
			ORDER BY wr.requested_at ASC`)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		list := []models.WithdrawalResponse{}
		for rows.Next() {
			var wr models.WithdrawalResponse
			var processedAt sql.NullTime
			if err := rows.Scan(&wr.ID, &wr.TeacherID, &wr.Amount, &wr.PaymentNumber, &wr.Method, &wr.Status, &wr.AdminNote, &wr.RequestedAt, &processedAt, &wr.TeacherName); err != nil {
				continue
			}
			if processedAt.Valid {
				wr.ProcessedAt = &processedAt.Time
			}
			list = append(list, wr)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "data": list})
	}
}

// ProcessWithdrawalHandler (admin) marks a withdrawal as paid (admin has
// manually sent the money via bKash/Nagad outside this system) or rejected
// (in which case the held amount is refunded back to the teacher's wallet).
func ProcessWithdrawalHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req models.ProcessWithdrawalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.WithdrawalID == 0 {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		if req.Status != "paid" && req.Status != "rejected" {
			http.Error(w, "Status must be 'paid' or 'rejected'", http.StatusBadRequest)
			return
		}

		tx, err := database.DB.Begin()
		if err != nil {
			http.Error(w, "Transaction error", http.StatusInternalServerError)
			return
		}

		var teacherID int
		var amount float64
		var currentStatus string
		err = tx.QueryRow(
			`SELECT teacher_id, amount, status FROM withdrawal_requests WHERE id = $1 FOR UPDATE`, req.WithdrawalID,
		).Scan(&teacherID, &amount, &currentStatus)
		if err == sql.ErrNoRows {
			tx.Rollback()
			http.Error(w, "Withdrawal request not found", http.StatusNotFound)
			return
		} else if err != nil {
			tx.Rollback()
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if currentStatus != "pending" {
			tx.Rollback()
			http.Error(w, "This request has already been processed", http.StatusBadRequest)
			return
		}

		_, err = tx.Exec(
			`UPDATE withdrawal_requests SET status = $1, admin_note = $2, processed_at = CURRENT_TIMESTAMP WHERE id = $3`,
			req.Status, req.AdminNote, req.WithdrawalID,
		)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to update withdrawal request", http.StatusInternalServerError)
			return
		}

		if req.Status == "rejected" {
			// Give the held amount back to the teacher's wallet.
			_, err = tx.Exec(`UPDATE users SET balance = balance + $1 WHERE id = $2`, amount, teacherID)
			if err != nil {
				tx.Rollback()
				http.Error(w, "Failed to refund teacher balance", http.StatusInternalServerError)
				return
			}
		}
		// If "paid": the money was already deducted from the teacher's balance
		// when they requested it, and the admin sent it manually outside this
		// system — nothing further to move here.

		if err := tx.Commit(); err != nil {
			http.Error(w, "Commit failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}
}
