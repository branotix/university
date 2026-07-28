package models

import "time"

// TopUpRequest ওয়ালেট রিচার্জের জন্য
type TopUpRequest struct {
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"payment_method"` // e.g., "bkash", "nagad"
	TrxID         string  `json:"trx_id"`
}

// PurchasePackageRequest কল সেশন কেনার জন্য
type PurchasePackageRequest struct {
	TeacherID      int     `json:"teacher_id"`
	PackageMinutes int     `json:"package_minutes"`
	Amount         float64 `json:"amount"`
}

type CallSessionResponse struct {
	SessionID      int       `json:"session_id"`
	StudentID      int       `json:"student_id"`
	TeacherID      int       `json:"teacher_id"`
	PackageMinutes int       `json:"package_minutes"`
	Amount         float64   `json:"amount"`
	Status         string    `json:"status"`
	StartedAt      time.Time `json:"started_at"`
}

type WithdrawalRequest struct {
	Amount        float64 `json:"amount"`
	PaymentNumber string  `json:"payment_number"`
	Method        string  `json:"method"`
}

type WithdrawalResponse struct {
	ID            int        `json:"id"`
	TeacherID     int        `json:"teacher_id"`
	Amount        float64    `json:"amount"`
	PaymentNumber string     `json:"payment_number"`
	Method        string     `json:"method"`
	Status        string     `json:"status"`
	AdminNote     string     `json:"admin_note"`
	RequestedAt   time.Time  `json:"requested_at"`
	ProcessedAt   *time.Time `json:"processed_at,omitempty"`
	TeacherName   string     `json:"teacher_name,omitempty"`
}

type ProcessWithdrawalRequest struct {
	WithdrawalID int    `json:"withdrawal_id"`
	Status       string `json:"status"` // "paid" or "rejected"
	AdminNote    string `json:"admin_note"`
}
