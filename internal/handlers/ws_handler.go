package handlers

import (
	"log"
	"net/http"
	"strconv"

	"varsity-network/internal/services"
	ws "varsity-network/internal/websocket"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func ServeWS(hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userIDStr := r.URL.Query().Get("user_id")
		userID, err := strconv.Atoi(userIDStr)
		if err != nil || userID == 0 {
			http.Error(w, "Invalid user_id", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade error:", err)
			return
		}

		client := &ws.Client{
			UserID: userID,
			Conn:   conn,
			Send:   make(chan ws.SignalMessage, 256),
		}

		hub.Register <- client

		go readPump(hub, client)
		go writePump(client)
	}
}

func readPump(hub *ws.Hub, client *ws.Client) {
	defer func() {
		hub.Unregister <- client
		client.Conn.Close()
	}()

	for {
		var msg ws.SignalMessage
		err := client.Conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("WS Read Error for user %d: %v", client.UserID, err)
			break
		}

		msg.SenderID = client.UserID

		// বিশেষ ইভেন্ট: যদি কল স্টার্টের মেসেজ আসে
		if msg.Type == "start_call" {
			dataMap, ok := msg.Data.(map[string]interface{})
			if ok {
				sessionID := int(dataMap["session_id"].(float64))
				packageMinutes := int(dataMap["package_minutes"].(float64))
				teacherID := msg.TargetID

				// ব্যাকগ্রাউন্ড টাইমার চালুকরণ
				services.StartCallTimer(hub, sessionID, client.UserID, teacherID, packageMinutes)
			}
		}

		// The callee answered: mark the session as actually connected. This is
		// what distinguishes "the call never connected" (always refunded) from
		// "the call connected and then someone hung up" (student/teacher rule
		// applies) — see services.FinalizeCallSession.
		if msg.Type == "answer" {
			dataMap, ok := msg.Data.(map[string]interface{})
			if ok {
				if sessionIDRaw, exists := dataMap["session_id"]; exists {
					if sessionIDFloat, ok := sessionIDRaw.(float64); ok {
						services.MarkSessionConnected(int(sessionIDFloat))
					}
				}
			}
		}

		hub.Broadcast <- msg
	}
}

func writePump(client *ws.Client) {
	defer client.Conn.Close()

	for msg := range client.Send {
		err := client.Conn.WriteJSON(msg)
		if err != nil {
			log.Printf("WS Write Error for user %d: %v", client.UserID, err)
			break
		}
	}
}
