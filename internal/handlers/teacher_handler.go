package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"varsity-network/internal/database"
	"varsity-network/internal/models"
)

// ApproveKYCHandler এডমিন কর্তৃক টিচারের KYC এপ্রুভ করার জন্য
func ApproveKYCHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req models.KYCApproveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		query := `UPDATE teacher_profiles SET kyc_status = $1 WHERE user_id = $2`
		res, err := database.DB.Exec(query, req.Status, req.TeacherUserID)
		if err != nil {
			http.Error(w, "Failed to update KYC status", http.StatusInternalServerError)
			return
		}

		rows, _ := res.RowsAffected()
		if rows == 0 {
			http.Error(w, "Teacher profile not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Teacher KYC status updated successfully",
		})
	}
}

// GetTeachersHandler ভার্সিটি ফিল্টারিং ও স্মার্ট র্যাংকিং অনুযায়ী টিচার লিস্ট দেখায়
func GetTeachersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		universityQuery := r.URL.Query().Get("university")

		baseQuery := `
			SELECT 
				u.id, u.name, u.gender, u.is_online,
				tp.university, tp.expertise, COALESCE(tp.bio, '') as bio,
				tp.girls_only_mode, tp.total_services_given,
				CASE WHEN tp.total_ratings > 0 THEN (tp.rating_sum / tp.total_ratings) ELSE 0.00 END as avg_rating
			FROM users u
			JOIN teacher_profiles tp ON u.id = tp.user_id
			WHERE tp.kyc_status = 'approved'`

		var dbRows *sql.Rows
		var err error

		if universityQuery != "" {
			baseQuery += ` AND LOWER(tp.university) = LOWER($1) ORDER BY u.is_online DESC, avg_rating DESC, tp.total_services_given DESC`
			dbRows, err = database.DB.Query(baseQuery, universityQuery)
		} else {
			baseQuery += ` ORDER BY u.is_online DESC, avg_rating DESC, tp.total_services_given DESC`
			dbRows, err = database.DB.Query(baseQuery)
		}

		if err != nil {
			http.Error(w, "Database query error", http.StatusInternalServerError)
			return
		}
		defer dbRows.Close()

		teachers := []models.TeacherProfileResponse{}
		for dbRows.Next() {
			var t models.TeacherProfileResponse
			err := dbRows.Scan(
				&t.UserID, &t.Name, &t.Gender, &t.IsOnline,
				&t.University, &t.Expertise, &t.Bio,
				&t.GirlsOnlyMode, &t.TotalServices, &t.AverageRating,
			)
			if err != nil {
				continue
			}
			teachers = append(teachers, t)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"count":  len(teachers),
			"data":   teachers,
		})
	}
}

// PendingTeacherResponse is what the admin panel shows for each teacher
// awaiting KYC review.
type PendingTeacherResponse struct {
	UserID         int    `json:"user_id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	University     string `json:"university"`
	Expertise      string `json:"expertise"`
	StudentCardURL string `json:"student_card_url"`
	KYCStatus      string `json:"kyc_status"`
}

// GetPendingTeachersHandler lists teacher accounts whose KYC hasn't been
// approved yet, so an admin can review the uploaded student ID card and
// approve or reject them. Admin-only (enforced at the route level).
func GetPendingTeachersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		query := `
			SELECT u.id, u.name, u.email, u.phone, tp.university, tp.expertise, tp.student_card_url, tp.kyc_status
			FROM users u
			JOIN teacher_profiles tp ON u.id = tp.user_id
			WHERE tp.kyc_status = 'pending'
			ORDER BY u.created_at ASC`

		rows, err := database.DB.Query(query)
		if err != nil {
			http.Error(w, "Database query error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		list := []PendingTeacherResponse{}
		for rows.Next() {
			var t PendingTeacherResponse
			if err := rows.Scan(&t.UserID, &t.Name, &t.Email, &t.Phone, &t.University, &t.Expertise, &t.StudentCardURL, &t.KYCStatus); err != nil {
				continue
			}
			list = append(list, t)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"count":  len(list),
			"data":   list,
		})
	}
}
