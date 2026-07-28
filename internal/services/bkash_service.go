package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"varsity-network/internal/config"
)

// BkashClient talks to bKash's Tokenized Checkout API (sandbox or live,
// depending on cfg.BkashBaseURL). It caches the grant token in memory and
// re-fetches it once it's close to expiring.
type BkashClient struct {
	cfg         *config.Config
	httpClient  *http.Client
	idToken     string
	tokenExpiry time.Time
}

func NewBkashClient(cfg *config.Config) *BkashClient {
	return &BkashClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type bkashGrantTokenResponse struct {
	StatusCode    string `json:"statusCode"`
	StatusMessage string `json:"statusMessage"`
	IDToken       string `json:"id_token"`
	TokenType     string `json:"token_type"`
	ExpiresIn     int    `json:"expires_in"`
	RefreshToken  string `json:"refresh_token"`
}

// GetToken returns a valid id_token, fetching a fresh one from bKash if the
// cached token is missing or about to expire.
func (b *BkashClient) GetToken() (string, error) {
	if b.idToken != "" && time.Now().Before(b.tokenExpiry) {
		return b.idToken, nil
	}

	body := map[string]string{
		"app_key":    b.cfg.BkashAppKey,
		"app_secret": b.cfg.BkashAppSecret,
	}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", b.cfg.BkashBaseURL+"/checkout/token/grant", bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("username", b.cfg.BkashUsername)
	req.Header.Set("password", b.cfg.BkashPassword)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)

	var tokenResp bkashGrantTokenResponse
	if err := json.Unmarshal(respBytes, &tokenResp); err != nil {
		return "", fmt.Errorf("bkash grant token: invalid response: %s", string(respBytes))
	}
	if tokenResp.IDToken == "" {
		return "", fmt.Errorf("bkash grant token failed: %s - %s", tokenResp.StatusCode, tokenResp.StatusMessage)
	}

	b.idToken = tokenResp.IDToken
	// bKash tokens are normally valid for 1 hour; refresh a bit early to be safe.
	b.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-120) * time.Second)

	return b.idToken, nil
}

type BkashCreatePaymentResponse struct {
	PaymentID     string `json:"paymentID"`
	BkashURL      string `json:"bkashURL"`
	StatusCode    string `json:"statusCode"`
	StatusMessage string `json:"statusMessage"`
}

// CreatePayment starts a new bKash payment session for the given amount and
// returns the paymentID + bkashURL the user should be redirected to.
func (b *BkashClient) CreatePayment(amount float64, merchantInvoiceNumber, callbackURL string) (*BkashCreatePaymentResponse, error) {
	token, err := b.GetToken()
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"mode":                  "0011",
		"payerReference":        "01700000000",
		"callbackURL":           callbackURL,
		"amount":                fmt.Sprintf("%.2f", amount),
		"currency":              "BDT",
		"intent":                "sale",
		"merchantInvoiceNumber": merchantInvoiceNumber,
	}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", b.cfg.BkashBaseURL+"/checkout/create", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", token)
	req.Header.Set("X-App-Key", b.cfg.BkashAppKey)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)

	var createResp BkashCreatePaymentResponse
	if err := json.Unmarshal(respBytes, &createResp); err != nil {
		return nil, fmt.Errorf("bkash create payment: invalid response: %s", string(respBytes))
	}
	if createResp.PaymentID == "" {
		return nil, fmt.Errorf("bkash create payment failed: %s - %s", createResp.StatusCode, createResp.StatusMessage)
	}

	return &createResp, nil
}

type BkashExecutePaymentResponse struct {
	PaymentID          string `json:"paymentID"`
	TrxID               string `json:"trxID"`
	TransactionStatus   string `json:"transactionStatus"`
	Amount              string `json:"amount"`
	StatusCode          string `json:"statusCode"`
	StatusMessage       string `json:"statusMessage"`
}

// ExecutePayment finalizes a payment after the user has approved it on the
// bKash-hosted page and bKash has redirected back to our callback URL.
func (b *BkashClient) ExecutePayment(paymentID string) (*BkashExecutePaymentResponse, error) {
	token, err := b.GetToken()
	if err != nil {
		return nil, err
	}

	body := map[string]string{"paymentID": paymentID}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", b.cfg.BkashBaseURL+"/checkout/execute", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", token)
	req.Header.Set("X-App-Key", b.cfg.BkashAppKey)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)

	var execResp BkashExecutePaymentResponse
	if err := json.Unmarshal(respBytes, &execResp); err != nil {
		return nil, fmt.Errorf("bkash execute payment: invalid response: %s", string(respBytes))
	}

	return &execResp, nil
}
