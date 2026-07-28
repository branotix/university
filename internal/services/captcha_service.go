package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"
)

type captchaEntry struct {
	answer    int
	expiresAt time.Time
}

var (
	captchaStore = make(map[string]captchaEntry)
	captchaMu    sync.Mutex
)

// GenerateCaptcha creates a simple "X + Y = ?" challenge, stores the answer
// server-side keyed by a random ID, and returns the ID + question text to
// show the user. This is a lightweight, self-hosted "I am not a robot"
// check — no external API key needed (unlike reCAPTCHA/hCaptcha).
func GenerateCaptcha() (id string, question string) {
	a, _ := rand.Int(rand.Reader, big.NewInt(8))
	b, _ := rand.Int(rand.Reader, big.NewInt(8))
	aInt := int(a.Int64()) + 2 // 2-9
	bInt := int(b.Int64()) + 2 // 2-9
	answer := aInt + bInt

	idBytes := make([]byte, 12)
	rand.Read(idBytes)
	id = hex.EncodeToString(idBytes)
	question = fmt.Sprintf("%d + %d = ?", aInt, bInt)

	captchaMu.Lock()
	captchaStore[id] = captchaEntry{answer: answer, expiresAt: time.Now().Add(5 * time.Minute)}
	cleanupExpiredCaptchasLocked()
	captchaMu.Unlock()

	return id, question
}

// VerifyCaptcha checks the answer and consumes the challenge (one-time use)
// regardless of whether it was correct, so it can't be brute-forced by
// retrying the same captcha_id.
func VerifyCaptcha(id string, answer int) bool {
	captchaMu.Lock()
	defer captchaMu.Unlock()

	entry, ok := captchaStore[id]
	delete(captchaStore, id)

	if !ok || time.Now().After(entry.expiresAt) {
		return false
	}
	return entry.answer == answer
}

func cleanupExpiredCaptchasLocked() {
	now := time.Now()
	for k, v := range captchaStore {
		if now.After(v.expiresAt) {
			delete(captchaStore, k)
		}
	}
}
