package websocket

import (
	"log"
	"net/http"
	"varsity-network/internal/database"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Production-এ নির্দিষ্ট CORS Origin সেট করতে পারেন
	},
}

// SignalMessage WebRTC মেসেজ ফরম্যাট
type SignalMessage struct {
	Type     string      `json:"type"`      // "offer", "answer", "candidate", "leave"
	TargetID int         `json:"target_id"` // প্রাপক ইউজারের ID
	SenderID int         `json:"sender_id"` // প্রেরক ইউজারের ID
	Data     interface{} `json:"data"`      // WebRTC SDP payload or ICE candidate
}

// Client একজন একক কানেক্টেড ইউজার
type Client struct {
	UserID int
	Conn   *websocket.Conn
	Send   chan SignalMessage
}

// Hub সকল একটিভ কানেকশন ধরে রাখে
type Hub struct {
	Clients    map[int]*Client
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan SignalMessage
}

func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[int]*Client),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan SignalMessage),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients[client.UserID] = client
			log.Printf("🔌 User Connected to WS: %d", client.UserID)

			// ১. ইউজার কানেক্ট হলে ডাটাবেসে is_online = true করা
			database.DB.Exec("UPDATE users SET is_online = true WHERE id = $1", client.UserID)

		case client := <-h.Unregister:
			if _, ok := h.Clients[client.UserID]; ok {
				delete(h.Clients, client.UserID)
				close(client.Send)
				log.Printf("❌ User Disconnected from WS: %d", client.UserID)

				// ২. ইউজার ডিসকানেক্ট হলে ডাটাবেসে is_online = false করা
				database.DB.Exec("UPDATE users SET is_online = false WHERE id = $1", client.UserID)
			}

		case message := <-h.Broadcast:
			if targetClient, ok := h.Clients[message.TargetID]; ok {
				select {
				case targetClient.Send <- message:
				default:
					close(targetClient.Send)
					delete(h.Clients, message.TargetID)
				}
			}
		}
	}
}
