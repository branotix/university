package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"varsity-network/internal/database"
	"varsity-network/internal/models"
	"varsity-network/internal/routes"
	"varsity-network/pkg/utils"
)

// GetMeHandler returns the logged-in user's own profile. The frontend uses this
// right after login (and on page refresh) to know who is logged in, their role,
// wallet balance, and — for teachers — their KYC status and stats.
func GetMeHandler() http.HandlerFunc {
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

		var me models.MeResponse
		query := `SELECT id, name, email, phone, role, COALESCE(gender, ''), balance, is_online, email_verified,
			COALESCE(profile_photo_url, ''), COALESCE(cover_photo_url, ''), COALESCE(headline, ''),
			COALESCE(about, ''), COALESCE(location, ''), COALESCE(languages, '')
			FROM users WHERE id = $1`
		err := database.DB.QueryRow(query, claims.UserID).Scan(
			&me.ID, &me.Name, &me.Email, &me.Phone, &me.Role, &me.Gender, &me.Balance, &me.IsOnline, &me.EmailVerified,
			&me.ProfilePhotoURL, &me.CoverPhotoURL, &me.Headline, &me.About, &me.Location, &me.Languages,
		)
		if err == sql.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if me.Role == "teacher" {
			teacherQuery := `
				SELECT university, expertise, COALESCE(bio, ''), kyc_status, girls_only_mode, total_services_given,
					CASE WHEN total_ratings > 0 THEN (rating_sum / total_ratings) ELSE 0.00 END as avg_rating
				FROM teacher_profiles WHERE user_id = $1`
			database.DB.QueryRow(teacherQuery, claims.UserID).Scan(
				&me.University, &me.Expertise, &me.Bio, &me.KYCStatus, &me.GirlsOnlyMode,
				&me.TotalServicesGiven, &me.AverageRating,
			)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   me,
		})
	}
}

// UpdateProfileHandler lets a logged-in user edit their own profile fields —
// this is what powers "complete your profile 100%" after registration.
func UpdateProfileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch && r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		claims, ok := r.Context().Value(routes.UserContextKey).(*utils.Claims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req models.UpdateProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		if req.Headline != nil {
			database.DB.Exec(`UPDATE users SET headline = $1 WHERE id = $2`, *req.Headline, claims.UserID)
		}
		if req.About != nil {
			database.DB.Exec(`UPDATE users SET about = $1 WHERE id = $2`, *req.About, claims.UserID)
		}
		if req.Location != nil {
			database.DB.Exec(`UPDATE users SET location = $1 WHERE id = $2`, *req.Location, claims.UserID)
		}
		if req.Languages != nil {
			database.DB.Exec(`UPDATE users SET languages = $1 WHERE id = $2`, *req.Languages, claims.UserID)
		}
		if req.ProfilePhotoURL != nil {
			database.DB.Exec(`UPDATE users SET profile_photo_url = $1 WHERE id = $2`, *req.ProfilePhotoURL, claims.UserID)
		}
		if req.CoverPhotoURL != nil {
			database.DB.Exec(`UPDATE users SET cover_photo_url = $1 WHERE id = $2`, *req.CoverPhotoURL, claims.UserID)
		}

		if claims.Role == "teacher" {
			if req.Bio != nil {
				database.DB.Exec(`UPDATE teacher_profiles SET bio = $1 WHERE user_id = $2`, *req.Bio, claims.UserID)
			}
			if req.Expertise != nil {
				database.DB.Exec(`UPDATE teacher_profiles SET expertise = $1 WHERE user_id = $2`, *req.Expertise, claims.UserID)
			}
			if req.University != nil {
				database.DB.Exec(`UPDATE teacher_profiles SET university = $1 WHERE user_id = $2`, *req.University, claims.UserID)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Profile updated."})
	}
}

// GetPublicProfileHandler returns a public, safe-to-share profile for any
// user by ID — this is what powers visiting someone's profile page.
func GetPublicProfileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid user id", http.StatusBadRequest)
			return
		}

		var p models.PublicProfileResponse
		query := `SELECT id, name, role, COALESCE(profile_photo_url,''), COALESCE(cover_photo_url,''),
			COALESCE(headline,''), COALESCE(about,''), COALESCE(location,''), COALESCE(languages,''),
			is_online, created_at::text
			FROM users WHERE id = $1`
		err = database.DB.QueryRow(query, id).Scan(
			&p.ID, &p.Name, &p.Role, &p.ProfilePhotoURL, &p.CoverPhotoURL,
			&p.Headline, &p.About, &p.Location, &p.Languages, &p.IsOnline, &p.JoinedAt,
		)
		if err == sql.ErrNoRows {
			http.Error(w, "Profile not found", http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if p.Role == "teacher" {
			var kycStatus string
			database.DB.QueryRow(`
				SELECT university, expertise, COALESCE(bio,''), girls_only_mode, total_services_given, kyc_status,
					CASE WHEN total_ratings > 0 THEN (rating_sum / total_ratings) ELSE 0.00 END
				FROM teacher_profiles WHERE user_id = $1`, id).Scan(
				&p.University, &p.Expertise, &p.Bio, &p.GirlsOnlyMode, &p.TotalServicesGiven, &kycStatus, &p.AverageRating,
			)
			p.KYCApproved = kycStatus == "approved"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "data": p})
	}
}

// GetFeedHandler lists public profiles for the home feed — both teachers and
// students, optionally filtered by role and/or university.
func GetFeedHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roleFilter := strings.ToLower(r.URL.Query().Get("role"))
		university := r.URL.Query().Get("university")

		query := `
			SELECT u.id, u.name, u.role, COALESCE(u.profile_photo_url,''), COALESCE(u.headline,''),
				COALESCE(u.location,''), u.is_online,
				COALESCE(tp.university,''), COALESCE(tp.expertise,''), COALESCE(tp.girls_only_mode,false),
				COALESCE(tp.total_services_given,0),
				COALESCE(CASE WHEN tp.total_ratings > 0 THEN (tp.rating_sum / tp.total_ratings) ELSE 0.00 END, 0.00)
			FROM users u
			LEFT JOIN teacher_profiles tp ON tp.user_id = u.id AND tp.kyc_status = 'approved'
			WHERE u.role != 'admin'
			AND (u.role != 'teacher' OR tp.kyc_status = 'approved')`

		args := []interface{}{}
		argN := 1
		if roleFilter == "student" || roleFilter == "teacher" {
			query += ` AND u.role = $` + strconv.Itoa(argN)
			args = append(args, roleFilter)
			argN++
		}
		if university != "" {
			query += ` AND tp.university = $` + strconv.Itoa(argN)
			args = append(args, university)
			argN++
		}
		query += ` ORDER BY u.is_online DESC, u.created_at DESC LIMIT 60`

		rows, err := database.DB.Query(query, args...)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type feedItem struct {
			ID                 int     `json:"id"`
			Name               string  `json:"name"`
			Role               string  `json:"role"`
			ProfilePhotoURL    string  `json:"profile_photo_url"`
			Headline           string  `json:"headline"`
			Location           string  `json:"location"`
			IsOnline           bool    `json:"is_online"`
			University         string  `json:"university,omitempty"`
			Expertise          string  `json:"expertise,omitempty"`
			GirlsOnlyMode      bool    `json:"girls_only_mode,omitempty"`
			TotalServicesGiven int     `json:"total_services_given,omitempty"`
			AverageRating      float64 `json:"average_rating,omitempty"`
		}

		list := []feedItem{}
		for rows.Next() {
			var f feedItem
			if err := rows.Scan(&f.ID, &f.Name, &f.Role, &f.ProfilePhotoURL, &f.Headline, &f.Location, &f.IsOnline,
				&f.University, &f.Expertise, &f.GirlsOnlyMode, &f.TotalServicesGiven, &f.AverageRating); err != nil {
				continue
			}
			list = append(list, f)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "data": list})
	}
}

// ToggleGirlsOnlyModeHandler lets a teacher turn "Girls Only Mode" on or off.
// When enabled, only female students should be able to call this teacher —
// the actual call-eligibility check happens in PurchaseCallPackageHandler.
func ToggleGirlsOnlyModeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodPatch {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		claims, ok := r.Context().Value(routes.UserContextKey).(*utils.Claims)
		if !ok || claims.Role != "teacher" {
			http.Error(w, "Only teachers can use this endpoint", http.StatusForbidden)
			return
		}

		var req models.ToggleGirlsOnlyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		query := `UPDATE teacher_profiles SET girls_only_mode = $1 WHERE user_id = $2`
		res, err := database.DB.Exec(query, req.Enabled, claims.UserID)
		if err != nil {
			http.Error(w, "Failed to update setting", http.StatusInternalServerError)
			return
		}

		rows, _ := res.RowsAffected()
		if rows == 0 {
			http.Error(w, "Teacher profile not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "success",
			"girls_only_mode": req.Enabled,
		})
	}
}
