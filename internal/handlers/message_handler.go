package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"varsity-network/internal/database"
	"varsity-network/internal/routes"
	"varsity-network/pkg/utils"
)

type sendMessageRequest struct {
	ReceiverID int    `json:"receiver_id"`
	Content    string `json:"content"`
}

// SendMessageHandler lets any logged-in user send a free text message to
// any other user (messaging is free — only calls are paid).
func SendMessageHandler() http.HandlerFunc {
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

		var req sendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ReceiverID == 0 || req.Content == "" {
			http.Error(w, "receiver_id and content are required", http.StatusBadRequest)
			return
		}
		if req.ReceiverID == claims.UserID {
			http.Error(w, "You can't message yourself", http.StatusBadRequest)
			return
		}

		var msgID int
		err := database.DB.QueryRow(
			`INSERT INTO messages (sender_id, receiver_id, content) VALUES ($1, $2, $3) RETURNING id`,
			claims.UserID, req.ReceiverID, req.Content,
		).Scan(&msgID)
		if err != nil {
			http.Error(w, "Failed to send message", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "message_id": msgID})
	}
}

// GetConversationsHandler lists the logged-in user's conversations: one row
// per other person they've exchanged messages with, with a preview of the
// last message and an unread count.
func GetConversationsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(routes.UserContextKey).(*utils.Claims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		query := `
			WITH convo AS (
				SELECT CASE WHEN sender_id = $1 THEN receiver_id ELSE sender_id END AS peer_id,
					   content, created_at, sender_id, read_at,
					   ROW_NUMBER() OVER (
						 PARTITION BY CASE WHEN sender_id = $1 THEN receiver_id ELSE sender_id END
						 ORDER BY created_at DESC
					   ) AS rn
				FROM messages
				WHERE sender_id = $1 OR receiver_id = $1
			)
			SELECT c.peer_id, u.name, COALESCE(u.profile_photo_url,''), u.role, u.is_online, c.content, c.created_at,
				(SELECT COUNT(*) FROM messages m WHERE m.sender_id = c.peer_id AND m.receiver_id = $1 AND m.read_at IS NULL) AS unread
			FROM convo c
			JOIN users u ON u.id = c.peer_id
			WHERE c.rn = 1
			ORDER BY c.created_at DESC`

		rows, err := database.DB.Query(query, claims.UserID)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type convoItem struct {
			PeerID          int    `json:"peer_id"`
			Name            string `json:"name"`
			ProfilePhotoURL string `json:"profile_photo_url"`
			Role            string `json:"role"`
			IsOnline        bool   `json:"is_online"`
			LastMessage     string `json:"last_message"`
			LastAt          string `json:"last_at"`
			Unread          int    `json:"unread"`
		}

		list := []convoItem{}
		for rows.Next() {
			var c convoItem
			var lastAt sql.NullTime
			if err := rows.Scan(&c.PeerID, &c.Name, &c.ProfilePhotoURL, &c.Role, &c.IsOnline, &c.LastMessage, &lastAt, &c.Unread); err != nil {
				continue
			}
			if lastAt.Valid {
				c.LastAt = lastAt.Time.Format("2006-01-02T15:04:05Z07:00")
			}
			list = append(list, c)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "data": list})
	}
}

// GetThreadHandler returns the full message thread with one specific peer,
// and marks their messages to us as read.
func GetThreadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(routes.UserContextKey).(*utils.Claims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		peerID, err := strconv.Atoi(r.PathValue("peer_id"))
		if err != nil {
			http.Error(w, "Invalid peer id", http.StatusBadRequest)
			return
		}

		rows, err := database.DB.Query(`
			SELECT id, sender_id, receiver_id, content, created_at
			FROM messages
			WHERE (sender_id = $1 AND receiver_id = $2) OR (sender_id = $2 AND receiver_id = $1)
			ORDER BY created_at ASC
			LIMIT 200`, claims.UserID, peerID)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type msgItem struct {
			ID         int    `json:"id"`
			SenderID   int    `json:"sender_id"`
			ReceiverID int    `json:"receiver_id"`
			Content    string `json:"content"`
			CreatedAt  string `json:"created_at"`
		}

		list := []msgItem{}
		for rows.Next() {
			var m msgItem
			var createdAt sql.NullTime
			if err := rows.Scan(&m.ID, &m.SenderID, &m.ReceiverID, &m.Content, &createdAt); err != nil {
				continue
			}
			if createdAt.Valid {
				m.CreatedAt = createdAt.Time.Format("2006-01-02T15:04:05Z07:00")
			}
			list = append(list, m)
		}

		database.DB.Exec(`UPDATE messages SET read_at = NOW() WHERE sender_id = $1 AND receiver_id = $2 AND read_at IS NULL`, peerID, claims.UserID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "data": list})
	}
}
