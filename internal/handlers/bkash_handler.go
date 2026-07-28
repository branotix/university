package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"varsity-network/internal/config"
	"varsity-network/internal/database"
	"varsity-network/internal/routes"
	"varsity-network/internal/services"
	"varsity-network/pkg/utils"
)

type bkashCreateTopUpRequest struct {
	Amount float64 `json:"amount"`
}

// CreateBkashPaymentHandler starts a bKash Tokenized Checkout payment for
// topping up the logged-in user's wallet. It returns a bkashURL the frontend
// should redirect the browser to, so the user can approve the payment on
// bKash's own page (this never touches the user's PIN on our server).
func CreateBkashPaymentHandler(cfg *config.Config) http.HandlerFunc {
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

		var req bkashCreateTopUpRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Amount <= 0 {
			http.Error(w, "Invalid top-up amount", http.StatusBadRequest)
			return
		}

		bkash := services.NewBkashClient(cfg)
		invoiceNumber := fmt.Sprintf("VN-%d-%d", claims.UserID, timeNowUnix())
		// callbackURL must point back to THIS backend server (bKash redirects the
		// user's browser here so we can verify + execute the payment).
		callbackURL := cfg.PublicBaseURL + "/api/wallet/bkash/callback"

		payment, err := bkash.CreatePayment(req.Amount, invoiceNumber, callbackURL)
		if err != nil {
			log.Println("bKash create payment error:", err)
			http.Error(w, "Failed to initiate bKash payment. Please try again.", http.StatusBadGateway)
			return
		}

		_, dbErr := database.DB.Exec(
			`INSERT INTO wallet_transactions (user_id, payment_id, amount, status) VALUES ($1, $2, $3, 'initiated')`,
			claims.UserID, payment.PaymentID, req.Amount,
		)
		if dbErr != nil {
			log.Println("Failed to record wallet_transactions row:", dbErr)
			http.Error(w, "Internal error while recording transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "success",
			"payment_id": payment.PaymentID,
			"bkash_url":  payment.BkashURL,
		})
	}
}

// BkashCallbackHandler is where bKash redirects the user's browser back to
// after they approve (or cancel) the payment. This is a public endpoint (no
// JWT — the browser navigates here directly), so we NEVER trust the amount
// from the query string. We look up our own wallet_transactions row by
// paymentID and verify the result directly with bKash via ExecutePayment.
func BkashCallbackHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		paymentID := r.URL.Query().Get("paymentID")
		bkashStatus := r.URL.Query().Get("status")

		if paymentID == "" {
			renderBkashResult(w, false, "Missing payment reference.")
			return
		}

		if bkashStatus == "cancel" || bkashStatus == "failure" {
			database.DB.Exec(`UPDATE wallet_transactions SET status = 'failed' WHERE payment_id = $1`, paymentID)
			renderBkashResult(w, false, "Payment was cancelled or failed.")
			return
		}

		bkash := services.NewBkashClient(cfg)
		result, err := bkash.ExecutePayment(paymentID)
		if err != nil {
			log.Println("bKash execute payment error:", err)
			database.DB.Exec(`UPDATE wallet_transactions SET status = 'failed' WHERE payment_id = $1`, paymentID)
			renderBkashResult(w, false, "Could not verify payment with bKash.")
			return
		}

		if result.TransactionStatus != "Completed" {
			database.DB.Exec(`UPDATE wallet_transactions SET status = 'failed' WHERE payment_id = $1`, paymentID)
			renderBkashResult(w, false, "Payment was not completed: "+result.StatusMessage)
			return
		}

		// Credit the wallet using the amount WE stored when the payment was
		// created (never the amount from the callback URL / bKash response),
		// and only if this transaction hasn't already been credited.
		tx, err := database.DB.Begin()
		if err != nil {
			renderBkashResult(w, false, "Internal error.")
			return
		}

		var userID int
		var amount float64
		var status string
		err = tx.QueryRow(`SELECT user_id, amount, status FROM wallet_transactions WHERE payment_id = $1 FOR UPDATE`, paymentID).
			Scan(&userID, &amount, &status)
		if err != nil {
			tx.Rollback()
			renderBkashResult(w, false, "Transaction record not found.")
			return
		}

		if status == "completed" {
			tx.Rollback()
			renderBkashResult(w, true, "Payment already processed.")
			return
		}

		_, err = tx.Exec(`UPDATE wallet_transactions SET status = 'completed', trx_id = $1 WHERE payment_id = $2`, result.TrxID, paymentID)
		if err != nil {
			tx.Rollback()
			renderBkashResult(w, false, "Failed to finalize transaction.")
			return
		}

		_, err = tx.Exec(`UPDATE users SET balance = balance + $1 WHERE id = $2`, amount, userID)
		if err != nil {
			tx.Rollback()
			renderBkashResult(w, false, "Failed to credit wallet.")
			return
		}

		if err := tx.Commit(); err != nil {
			renderBkashResult(w, false, "Failed to commit transaction.")
			return
		}

		renderBkashResult(w, true, fmt.Sprintf("Payment successful! %.2f BDT added to your wallet.", amount))
	}
}

// renderBkashResult shows a minimal HTML page after the bKash redirect,
// which auto-closes / links back to the wallet page in the SPA.
func renderBkashResult(w http.ResponseWriter, success bool, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	icon := "✅"
	if !success {
		icon = "❌"
	}
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Payment Result</title>
<style>body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#0f172a;color:#f1f5f9;text-align:center}
.card{background:#1e293b;padding:2rem 2.5rem;border-radius:16px;max-width:420px}
a{color:#38bdf8}</style></head>
<body><div class="card"><div style="font-size:48px">%s</div><p>%s</p>
<a href="/student.html">প্ল্যাটফর্মে ফিরে যান</a></div></body></html>`, icon, message)
}

func timeNowUnix() int64 {
	return time.Now().Unix()
}
