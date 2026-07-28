package services

import (
	"fmt"
	"log"
	"math/rand"
	"net/smtp"

	"varsity-network/internal/config"
)

// GenerateVerificationCode returns a random 6-digit code as a string.
func GenerateVerificationCode() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

// SendVerificationEmail sends a 6-digit verification code to the given
// address using Gmail's SMTP server (or whatever SMTP_HOST is configured).
//
// IMPORTANT: Gmail rejects your normal account password over SMTP. You must
// create a 16-character "App Password" for this: Google Account -> Security
// -> 2-Step Verification (must be ON) -> App passwords. Put that (not your
// real Gmail password) in SMTP_PASSWORD.
//
// If SMTP_USER is not configured (e.g. during local development before you've
// set up Gmail), this falls back to just logging the code to the server
// console so you can still test the verification flow end-to-end. Remove/
// disable that fallback before real users sign up — see the README.
func SendVerificationEmail(cfg *config.Config, toEmail, code string) error {
	if cfg.SMTPUser == "" || cfg.SMTPPassword == "" {
		log.Printf("⚠️  SMTP not configured — DEV FALLBACK: verification code for %s is: %s", toEmail, code)
		return nil
	}

	from := cfg.SMTPFrom
	if from == "" {
		from = cfg.SMTPUser
	}

	subject := "Subject: Varsity Network — Verify your email\r\n"
	mime := "MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n"
	body := fmt.Sprintf(
		"Your Varsity Network verification code is: %s\r\n\r\nThis code expires in 15 minutes. If you didn't request this, you can ignore this email.\r\n",
		code,
	)
	msg := []byte(subject + mime + "\r\n" + body)

	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
	addr := cfg.SMTPHost + ":" + cfg.SMTPPort

	err := smtp.SendMail(addr, auth, from, []string{toEmail}, msg)
	if err != nil {
		log.Printf("❌ Failed to send verification email to %s: %v", toEmail, err)
		return err
	}

	log.Printf("✅ Verification email sent to %s", toEmail)
	return nil
}
