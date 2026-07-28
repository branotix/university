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

// TopUpWalletHandler ওয়ালেট ব্যালেন্স যোগ করার জন্য
func TopUpWalletHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		claims, ok := r.Context().Value(routes.UserContextKey).(*utils.Claims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req models.TopUpRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Amount <= 0 {
			http.Error(w, "Invalid top-up amount", http.StatusBadRequest)
			return
		}

		// Balance Update Query
		query := `UPDATE users SET balance = balance + $1 WHERE id = $2 RETURNING balance`
		var newBalance float64
		err := database.DB.QueryRow(query, req.Amount, claims.UserID).Scan(&newBalance)
		if err != nil {
			http.Error(w, "Failed to update balance", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "success",
			"message":     "Wallet topped up successfully",
			"new_balance": newBalance,
		})
	}
}

// PurchaseCallPackageHandler কল প্যাকেজ কেনা ও সেশন অ্যাক্টিভ করার জন্য
func PurchaseCallPackageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		claims, ok := r.Context().Value(routes.UserContextKey).(*utils.Claims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req models.PurchasePackageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Amount <= 0 || req.PackageMinutes <= 0 {
			http.Error(w, "Invalid package details", http.StatusBadRequest)
			return
		}

		// Eligibility check: teacher's Girls Only Mode + block list
		var studentGender string
		database.DB.QueryRow(`SELECT COALESCE(gender, '') FROM users WHERE id = $1`, claims.UserID).Scan(&studentGender)

		var girlsOnly bool
		errElig := database.DB.QueryRow(`SELECT girls_only_mode FROM teacher_profiles WHERE user_id = $1 AND kyc_status = 'approved'`, req.TeacherID).Scan(&girlsOnly)
		if errElig == sql.ErrNoRows {
			http.Error(w, "Teacher not found or not yet approved", http.StatusBadRequest)
			return
		}
		if girlsOnly && studentGender != "female" {
			http.Error(w, "This teacher is only accepting calls from female students right now", http.StatusForbidden)
			return
		}

		// Transaction Start (Atomicity)
		tx, err := database.DB.Begin()
		if err != nil {
			http.Error(w, "Transaction error", http.StatusInternalServerError)
			return
		}

		// ১. স্টুডেন্টের ব্যালেন্স চেক করা
		var currentBalance float64
		checkBalanceQuery := `SELECT balance FROM users WHERE id = $1 FOR UPDATE`
		err = tx.QueryRow(checkBalanceQuery, claims.UserID).Scan(&currentBalance)
		if err != nil {
			tx.Rollback()
			http.Error(w, "User not found", http.StatusBadRequest)
			return
		}

		if currentBalance < req.Amount {
			tx.Rollback()
			http.Error(w, "Insufficient wallet balance. Please top-up first.", http.StatusBadRequest)
			return
		}

		// ২. ব্যালেন্স কর্তন
		deductQuery := `UPDATE users SET balance = balance - $1 WHERE id = $2`
		_, err = tx.Exec(deductQuery, req.Amount, claims.UserID)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to deduct balance", http.StatusInternalServerError)
			return
		}

		// ৩. call_sessions টেবিলে কল এন্ট্রি তৈরি
		var session models.CallSessionResponse
		createSessionQuery := `
			INSERT INTO call_sessions (student_id, teacher_id, package_minutes, amount, status) 
			VALUES ($1, $2, $3, $4, 'active') 
			RETURNING id, student_id, teacher_id, package_minutes, amount, status, started_at`

		err = tx.QueryRow(createSessionQuery, claims.UserID, req.TeacherID, req.PackageMinutes, req.Amount).
			Scan(&session.SessionID, &session.StudentID, &session.TeacherID, &session.PackageMinutes, &session.Amount, &session.Status, &session.StartedAt)

		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to create call session", http.StatusInternalServerError)
			return
		}

		// Commit Transaction
		if err := tx.Commit(); err != nil {
			http.Error(w, "Commit failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "Call package purchased successfully!",
			"data":    session,
		})
	}
}
