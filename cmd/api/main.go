package main

import (
	"fmt"
	"log"
	"net/http"

	"varsity-network/internal/config"
	"varsity-network/internal/database"
	"varsity-network/internal/handlers"
	"varsity-network/internal/routes"
	ws "varsity-network/internal/websocket"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ Warning: No .env file found")
	}

	cfg := config.LoadConfig()
	db := database.InitDB(cfg)
	defer db.Close()

	// 1. Initialize WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()

	mux := http.NewServeMux()

	// Public Routes
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "success", "message": "Backend & DB live!"}`))
	})

	mux.HandleFunc("/api/register", handlers.RegisterHandler(cfg))
	mux.HandleFunc("/api/login", handlers.LoginHandler(cfg))
	mux.HandleFunc("/api/verify-email", handlers.VerifyEmailHandler(cfg))
	mux.HandleFunc("/api/resend-verification", handlers.ResendVerificationHandler(cfg))
	mux.HandleFunc("/api/teachers", handlers.GetTeachersHandler())
	mux.HandleFunc("/api/feed", handlers.GetFeedHandler())
	mux.HandleFunc("GET /api/profiles/{id}", handlers.GetPublicProfileHandler())
	mux.HandleFunc("/api/upload", handlers.UploadImageHandler())
	mux.HandleFunc("/api/captcha", handlers.GetCaptchaHandler())
	mux.HandleFunc("/api/stats", handlers.GetPlatformStatsHandler())

	// bKash callback is public — bKash redirects the user's browser here directly (no JWT available)
	mux.HandleFunc("/api/wallet/bkash/callback", handlers.BkashCallbackHandler(cfg))

	// Protected Routes (require a valid JWT, via Authorization header or the vn_token cookie)
	mux.HandleFunc("GET /api/me", routes.AuthMiddleware(cfg, handlers.GetMeHandler()))
	mux.HandleFunc("PATCH /api/me", routes.AuthMiddleware(cfg, handlers.UpdateProfileHandler()))
	mux.HandleFunc("/api/wallet/topup", routes.AuthMiddleware(cfg, handlers.TopUpWalletHandler()))
	mux.HandleFunc("/api/wallet/bkash/create", routes.AuthMiddleware(cfg, handlers.CreateBkashPaymentHandler(cfg)))
	mux.HandleFunc("/api/packages/purchase", routes.AuthMiddleware(cfg, handlers.PurchaseCallPackageHandler()))

	// Messaging (free)
	mux.HandleFunc("POST /api/messages", routes.AuthMiddleware(cfg, handlers.SendMessageHandler()))
	mux.HandleFunc("GET /api/messages/conversations", routes.AuthMiddleware(cfg, handlers.GetConversationsHandler()))
	mux.HandleFunc("GET /api/messages/thread/{peer_id}", routes.AuthMiddleware(cfg, handlers.GetThreadHandler()))

	// Teacher-only routes
	mux.HandleFunc("/api/teacher/girls-only-mode", routes.AuthMiddleware(cfg, handlers.ToggleGirlsOnlyModeHandler()))
	mux.HandleFunc("/api/teacher/withdraw", routes.AuthMiddleware(cfg, handlers.RequestWithdrawalHandler()))
	mux.HandleFunc("/api/teacher/withdrawals", routes.AuthMiddleware(cfg, handlers.GetMyWithdrawalsHandler()))

	// Call session lifecycle (payment/refund is finalized here, not in the review handler)
	mux.HandleFunc("/api/sessions/end", routes.AuthMiddleware(cfg, handlers.EndCallSessionHandler(hub)))
	mux.HandleFunc("/api/turn-credentials", routes.AuthMiddleware(cfg, handlers.GetTurnCredentialsHandler(cfg)))

	// Admin-only routes
	mux.HandleFunc("/api/admin/approve-kyc", routes.AuthMiddleware(cfg, routes.RequireRole("admin")(handlers.ApproveKYCHandler())))
	mux.HandleFunc("/api/admin/pending-teachers", routes.AuthMiddleware(cfg, routes.RequireRole("admin")(handlers.GetPendingTeachersHandler())))
	mux.HandleFunc("/api/admin/withdrawals/pending", routes.AuthMiddleware(cfg, routes.RequireRole("admin")(handlers.GetPendingWithdrawalsHandler())))
	mux.HandleFunc("/api/admin/withdrawals/process", routes.AuthMiddleware(cfg, routes.RequireRole("admin")(handlers.ProcessWithdrawalHandler())))

	// Rating & Review Route
	mux.HandleFunc("/api/reviews/submit", routes.AuthMiddleware(cfg, handlers.SubmitReviewHandler()))

	// WebSocket Signaling Route for WebRTC
	mux.HandleFunc("/ws", handlers.ServeWS(hub))

	// Web UI Serve (static frontend)
	mux.Handle("/", http.FileServer(http.Dir("./web")))

	handler := routes.WithCORS(cfg.FrontendOrigin, mux)

	fmt.Printf("🚀 Server running on port %s...\n", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
