package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"varsity-network/internal/config"
	"varsity-network/internal/database"
	"varsity-network/internal/models"
	"varsity-network/internal/services"
	"varsity-network/pkg/utils"
)

const verificationCodeTTL = 15 * time.Minute
const sessionCookieMaxAge = 30 * 24 * 60 * 60 // 30 days, in seconds

// setSessionCookie issues the persistent login cookie (in addition to the
// token returned in the JSON body, which the frontend also keeps in
// localStorage as a fallback for cross-origin/dev setups).
func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "vn_token",
		Value:    token,
		Path:     "/",
		MaxAge:   sessionCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// RegisterHandler ইউজার (Student & Teacher) রেজিস্ট্রেশন প্রসেস করে
func RegisterHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req models.RegisterRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, "Invalid input payload", http.StatusBadRequest)
			return
		}

		// Registration-e keu nijeke shorashori "admin" banate parbe na.
		// Admin account shudhu database-e manually toiri korte hobe.
		if req.Role != "student" && req.Role != "teacher" {
			http.Error(w, "Role must be either 'student' or 'teacher'", http.StatusBadRequest)
			return
		}

		if req.Name == "" || req.Email == "" || req.Phone == "" || len(req.Password) < 6 {
			http.Error(w, "Name, email, phone are required and password must be at least 6 characters", http.StatusBadRequest)
			return
		}

		if req.Role == "teacher" && (req.University == "" || req.StudentCardURL == "") {
			http.Error(w, "Teachers must upload their student ID card and provide their university", http.StatusBadRequest)
			return
		}

		if !services.VerifyCaptcha(req.CaptchaID, req.CaptchaAnswer) {
			http.Error(w, "Captcha verification failed. Please try again.", http.StatusBadRequest)
			return
		}

		hashedPassword, err := utils.HashPassword(req.Password)
		if err != nil {
			http.Error(w, "Error processing password", http.StatusInternalServerError)
			return
		}

		// Transaction শুরু (যেন Error হলে সব Rollback হয়ে যায়)
		tx, err := database.DB.Begin()
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		var userID int
		userQuery := `
			INSERT INTO users (name, email, phone, password_hash, role, gender) 
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

		err = tx.QueryRow(userQuery, req.Name, req.Email, req.Phone, hashedPassword, req.Role, req.Gender).Scan(&userID)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Email or Phone already exists", http.StatusBadRequest)
			return
		}

		// যদি ইউজার Teacher হয়, তবে teacher_profiles টেবিলেও ডাটা সেভ হবে
		if req.Role == "teacher" {
			teacherQuery := `
				INSERT INTO teacher_profiles (user_id, university, expertise, student_card_url, kyc_status) 
				VALUES ($1, $2, $3, $4, 'pending')`

			_, err = tx.Exec(teacherQuery, userID, req.University, req.Expertise, req.StudentCardURL)
			if err != nil {
				tx.Rollback()
				http.Error(w, "Failed to save teacher profile data", http.StatusInternalServerError)
				return
			}
		}

		code := services.GenerateVerificationCode()
		_, err = tx.Exec(
			`INSERT INTO email_verifications (user_id, code, expires_at) VALUES ($1, $2, $3)`,
			userID, code, time.Now().Add(verificationCodeTTL),
		)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to create verification code", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Transaction commit failed", http.StatusInternalServerError)
			return
		}

		go services.SendVerificationEmail(cfg, req.Email, code)

		resp := map[string]interface{}{
			"status":  "success",
			"message": "Account created! Check your email for a 6-digit verification code before you can log in.",
			"email":   req.Email,
		}
		// DEV CONVENIENCE ONLY: if SMTP isn't configured yet, hand back the code
		// directly so you can test the flow before setting up Gmail. Remove this
		// before letting real users register — see README.
		if cfg.SMTPUser == "" {
			resp["dev_code"] = code
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}

// VerifyEmailHandler confirms a user's 6-digit email verification code.
func VerifyEmailHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Email string `json:"email"`
			Code  string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Code == "" {
			http.Error(w, "Email and code are required", http.StatusBadRequest)
			return
		}

		var userID int
		err := database.DB.QueryRow(`SELECT id FROM users WHERE email = $1`, req.Email).Scan(&userID)
		if err == sql.ErrNoRows {
			http.Error(w, "No account found with that email", http.StatusBadRequest)
			return
		} else if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		var codeID int
		err = database.DB.QueryRow(
			`SELECT id FROM email_verifications WHERE user_id = $1 AND code = $2 AND expires_at > NOW() ORDER BY id DESC LIMIT 1`,
			userID, req.Code,
		).Scan(&codeID)
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid or expired code", http.StatusBadRequest)
			return
		} else if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if _, err := database.DB.Exec(`UPDATE users SET email_verified = true WHERE id = $1`, userID); err != nil {
			http.Error(w, "Failed to verify email", http.StatusInternalServerError)
			return
		}
		database.DB.Exec(`DELETE FROM email_verifications WHERE user_id = $1`, userID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Email verified! You can now log in.",
		})
	}
}

// ResendVerificationHandler issues a fresh code if the old one expired.
// Rate-limited to one send per minute per account, and gated behind the
// same captcha as registration, so nobody can spam an inbox with codes.
func ResendVerificationHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Email         string `json:"email"`
			CaptchaID     string `json:"captcha_id"`
			CaptchaAnswer int    `json:"captcha_answer"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
			http.Error(w, "Email is required", http.StatusBadRequest)
			return
		}

		if !services.VerifyCaptcha(req.CaptchaID, req.CaptchaAnswer) {
			http.Error(w, "Captcha verification failed. Please try again.", http.StatusBadRequest)
			return
		}

		var userID int
		var alreadyVerified bool
		err := database.DB.QueryRow(`SELECT id, email_verified FROM users WHERE email = $1`, req.Email).Scan(&userID, &alreadyVerified)
		if err != nil {
			// Don't reveal whether the email exists.
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "If that email is registered, a new code has been sent."})
			return
		}
		if alreadyVerified {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "This email is already verified — you can log in."})
			return
		}

		// Rate limit: at most one code every 60 seconds per account.
		var lastSentAt time.Time
		err = database.DB.QueryRow(
			`SELECT created_at FROM email_verifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID,
		).Scan(&lastSentAt)
		if err == nil {
			secondsLeft := 60 - int(time.Since(lastSentAt).Seconds())
			if secondsLeft > 0 {
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":          "error",
					"message":         "Please wait before requesting another code.",
					"retry_after_sec": secondsLeft,
				})
				return
			}
		}

		code := services.GenerateVerificationCode()
		database.DB.Exec(`DELETE FROM email_verifications WHERE user_id = $1`, userID)
		database.DB.Exec(
			`INSERT INTO email_verifications (user_id, code, expires_at) VALUES ($1, $2, $3)`,
			userID, code, time.Now().Add(verificationCodeTTL),
		)
		go services.SendVerificationEmail(cfg, req.Email, code)

		resp := map[string]interface{}{"status": "success", "message": "A new verification code has been sent."}
		if cfg.SMTPUser == "" {
			resp["dev_code"] = code
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// LoginHandler ইউজার লগইন করে টোকেন ব্যাক পাঠায়
func LoginHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req models.LoginRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, "Invalid input payload", http.StatusBadRequest)
			return
		}

		var user models.User
		var passwordHash string
		var emailVerified bool

		query := `SELECT id, name, email, password_hash, role, email_verified FROM users WHERE email = $1`
		err = database.DB.QueryRow(query, req.Email).Scan(&user.ID, &user.Name, &user.Email, &passwordHash, &user.Role, &emailVerified)
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		} else if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		// পাসওয়ার্ড চেক
		if !utils.CheckPasswordHash(req.Password, passwordHash) {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		if !emailVerified {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":           "error",
				"message":          "Please verify your email before logging in.",
				"needs_verification": true,
				"email":            user.Email,
			})
			return
		}

		// JWT Token জেনারেট
		token, err := utils.GenerateJWT(user.ID, user.Role, cfg.JWTSecret)
		if err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		setSessionCookie(w, token)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.AuthResponse{
			Status:  "success",
			Token:   token,
			Role:    user.Role,
			Message: "Login successful",
		})
	}
}
