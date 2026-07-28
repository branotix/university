package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	FrontendOrigin string
	PublicBaseURL  string
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	JWTSecret      string

	BkashBaseURL   string
	BkashAppKey    string
	BkashAppSecret string
	BkashUsername  string
	BkashPassword  string

	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

	TurnDomain string
	TurnSecret string
}

// LoadConfig env ভ্যালুগুলো লোড করে Struct-এ সেট করে
func LoadConfig() *Config {
	// .env ফাইল auto-load
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️ Warning: .env file not found, using fallback values")
	}

	return &Config{
		Port:           getEnv("PORT", "8080"),
		FrontendOrigin: getEnv("FRONTEND_ORIGIN", "*"),
		PublicBaseURL:  getEnv("PUBLIC_BASE_URL", "http://localhost:"+getEnv("PORT", "8080")),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "postgres"),     // fallback এখন myuser
		DBPassword:     getEnv("DB_PASSWORD", "postgres"), // fallback এখন mypassword
		DBName:         getEnv("DB_NAME", "varsity_db"),
		JWTSecret:      getEnv("JWT_SECRET", "default_secret_key"),

		BkashBaseURL:   getEnv("BKASH_BASE_URL", "https://tokenized.sandbox.bka.sh/v1.2.0-beta/tokenized"),
		BkashAppKey:    getEnv("BKASH_APP_KEY", ""),
		BkashAppSecret: getEnv("BKASH_APP_SECRET", ""),
		BkashUsername:  getEnv("BKASH_USERNAME", ""),
		BkashPassword:  getEnv("BKASH_PASSWORD", ""),

		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUser:     getEnv("SMTP_USER", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", ""),

		TurnDomain: getEnv("TURN_DOMAIN", ""),
		TurnSecret: getEnv("TURN_SECRET", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
