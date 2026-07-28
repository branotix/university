package handlers

import (
	"encoding/json"
	"net/http"

	"varsity-network/internal/config"
	"varsity-network/internal/routes"
	"varsity-network/internal/services"
	"varsity-network/pkg/utils"
)

// GetTurnCredentialsHandler hands back fresh, time-limited TURN credentials
// for the logged-in user to use in their WebRTC ICE_SERVERS list. Requires
// auth so random visitors can't harvest credentials and abuse your TURN
// server's bandwidth for unrelated traffic.
func GetTurnCredentialsHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(routes.UserContextKey).(*utils.Claims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		creds, configured := services.GenerateTurnCredentials(cfg, claims.UserID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "success",
			"configured": configured,
			"turn":      creds,
		})
	}
}
