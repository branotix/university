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

// SubmitReviewHandler records a student's rating/review for a completed call.
//
// NOTE: this handler does NOT move any money. Payment (or refund) is settled
// exactly once by services.FinalizeCallSession when the call actually ends
// (either side hanging up, or the package time running out) — see
// internal/services/call_service.go. Tying payment to "did the student
// bother to leave a review" was the bug that caused teachers to never get
// paid when a student skipped the review step.
func SubmitReviewHandler() http.HandlerFunc {
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

		var req models.ReviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Rating < 1 || req.Rating > 5 {
			http.Error(w, "Invalid rating. Must be between 1 and 5.", http.StatusBadRequest)
			return
		}

		tx, err := database.DB.Begin()
		if err != nil {
			http.Error(w, "Transaction error", http.StatusInternalServerError)
			return
		}

		// Make sure this session belongs to this student and has actually ended
		// (you can review a refunded session too — e.g. to flag a teacher who
		// keeps hanging up early — just not one that's still active).
		var sessionStatus string
		checkQuery := `SELECT status FROM call_sessions WHERE id = $1 AND student_id = $2`
		err = tx.QueryRow(checkQuery, req.SessionID, claims.UserID).Scan(&sessionStatus)
		if err == sql.ErrNoRows {
			tx.Rollback()
			http.Error(w, "Call session not found or unauthorized", http.StatusBadRequest)
			return
		} else if err != nil {
			tx.Rollback()
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if sessionStatus == "active" {
			tx.Rollback()
			http.Error(w, "This call hasn't ended yet", http.StatusBadRequest)
			return
		}

		insertReviewQuery := `
			INSERT INTO reviews (session_id, student_id, teacher_id, rating, comment)
			VALUES ($1, $2, $3, $4, $5)`
		_, err = tx.Exec(insertReviewQuery, req.SessionID, claims.UserID, req.TeacherID, req.Rating, req.Comment)
		if err != nil {
			tx.Rollback()
			http.Error(w, "You have already reviewed this session", http.StatusBadRequest)
			return
		}

		updateTeacherRatingQuery := `
			UPDATE teacher_profiles
			SET rating_sum = rating_sum + $1,
			    total_ratings = total_ratings + 1
			WHERE user_id = $2`
		_, err = tx.Exec(updateTeacherRatingQuery, req.Rating, req.TeacherID)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to update teacher rating", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Transaction commit failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "Review submitted successfully!",
		})
	}
}
