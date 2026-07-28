package handlers

import (
	"encoding/json"
	"net/http"

	"varsity-network/internal/services"
)

// GetCaptchaHandler issues a fresh "I am not a robot" math challenge.
// Public — needed before the user has an account.
func GetCaptchaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id, question := services.GenerateCaptcha()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"captcha_id": id,
			"question":   question,
		})
	}
}
