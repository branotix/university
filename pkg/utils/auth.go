package utils

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// HashPassword পাসওয়ার্ড হ্যাশ করে
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash হ্যাশকৃত পাসওয়ার্ডের সাথে ইনপুট ম্যাচ করে
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Claims Struct
type Claims struct {
	UserID int    `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateJWT টোকেন তৈরি করে
func GenerateJWT(userID int, role, secretKey string) (string, error) {
	expirationTime := time.Now().Add(30 * 24 * time.Hour) // ২৪ ঘণ্টার মেয়াদি টোকেন
	claims := &Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}
