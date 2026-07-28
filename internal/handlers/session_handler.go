package handlers

import (
	"encoding/json"
	"net/http"

	"varsity-network/internal/database"
	"varsity-network/internal/routes"
	"varsity-network/internal/services"
	ws "varsity-network/internal/websocket"
	"varsity-network/pkg/utils"
)

type endSessionRequest struct {
	SessionID int `json:"session_id"`
}

// EndCallSessionHandler is called by whichever participant actually clicks
// "end call" (or declines/cancels before it connects). It derives who ended
// the call from the authenticated user's own identity — never from a
// client-supplied "ended_by" field — so the payout/refund decision can't be
// spoofed by the losing side of the call.
func EndCallSessionHandler(hub *ws.Hub) http.HandlerFunc {
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

		var req endSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == 0 {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		var studentID, teacherID int
		err := database.DB.QueryRow(
			`SELECT student_id, teacher_id FROM call_sessions WHERE id = $1`, req.SessionID,
		).Scan(&studentID, &teacherID)
		if err != nil {
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}

		var endedBy string
		switch claims.UserID {
		case studentID:
			endedBy = "student"
		case teacherID:
			endedBy = "teacher"
		default:
			http.Error(w, "You are not part of this call session", http.StatusForbidden)
			return
		}

		if err := services.FinalizeCallSession(hub, req.SessionID, studentID, teacherID, endedBy); err != nil {
			http.Error(w, "Failed to finalize session", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}
}
