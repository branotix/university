package database

import (
	"database/sql"
	"fmt"
	"log"

	"varsity-network/internal/config"

	_ "github.com/lib/pq" // PostgreSQL driver
)

var DB *sql.DB

// InitDB ডাটাবেস কানেকশন তৈরি করে এবং টেবিলগুলো তৈরি করে
func InitDB(cfg *config.Config) *sql.DB {
	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)

	var err error
	DB, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatalf("❌ Database connection failed: %v", err)
	}

	err = DB.Ping()
	if err != nil {
		log.Fatalf("❌ Database is unreachable: %v", err)
	}

	fmt.Println("✅ Successfully connected to PostgreSQL Database!")

	// Auto Migration / Table creation
	createTables()

	return DB
}

func createTables() {
	queries := []string{
		// 1. Users Table
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			email VARCHAR(100) UNIQUE NOT NULL,
			phone VARCHAR(20) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			role VARCHAR(20) CHECK (role IN ('student', 'teacher', 'admin')) NOT NULL DEFAULT 'student',
			gender VARCHAR(10) CHECK (gender IN ('male', 'female', 'other')),
			balance NUMERIC(10, 2) DEFAULT 0.00,
			is_online BOOLEAN DEFAULT false,
			email_verified BOOLEAN DEFAULT false,
			profile_photo_url TEXT,
			cover_photo_url TEXT,
			headline VARCHAR(150),
			about TEXT,
			location VARCHAR(100),
			languages VARCHAR(200),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,
		// Safe migrations for databases created before these columns existed
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN DEFAULT false;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_photo_url TEXT;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS cover_photo_url TEXT;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS headline VARCHAR(150);`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS about TEXT;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS location VARCHAR(100);`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS languages VARCHAR(200);`,

		// 2. Teacher Profiles Table
		`CREATE TABLE IF NOT EXISTS teacher_profiles (
			id SERIAL PRIMARY KEY,
			user_id INT UNIQUE REFERENCES users(id) ON DELETE CASCADE,
			university VARCHAR(100) NOT NULL,
			expertise TEXT NOT NULL,
			bio TEXT,
			student_card_url TEXT NOT NULL,
			kyc_status VARCHAR(20) CHECK (kyc_status IN ('pending', 'approved', 'rejected')) DEFAULT 'pending',
			girls_only_mode BOOLEAN DEFAULT false,
			total_services_given INT DEFAULT 0,
			rating_sum NUMERIC(10, 2) DEFAULT 0.00,
			total_ratings INT DEFAULT 0
		);`,

		// 3. Call Sessions Table
		`CREATE TABLE IF NOT EXISTS call_sessions (
			id SERIAL PRIMARY KEY,
			student_id INT REFERENCES users(id),
			teacher_id INT REFERENCES users(id),
			package_minutes INT NOT NULL,
			amount NUMERIC(10, 2) NOT NULL,
			status VARCHAR(20) CHECK (status IN ('active', 'completed', 'cancelled', 'refunded')) DEFAULT 'active',
			ended_by VARCHAR(20) CHECK (ended_by IN ('timeout', 'student', 'teacher')),
			is_connected BOOLEAN DEFAULT false,
			started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			ended_at TIMESTAMP WITH TIME ZONE
		);`,
		// Safe migration for databases created before this column/constraint existed
		`ALTER TABLE call_sessions ADD COLUMN IF NOT EXISTS ended_by VARCHAR(20);`,
		`ALTER TABLE call_sessions ADD COLUMN IF NOT EXISTS is_connected BOOLEAN DEFAULT false;`,
		`DO $$
		BEGIN
			ALTER TABLE call_sessions DROP CONSTRAINT IF EXISTS call_sessions_status_check;
			ALTER TABLE call_sessions ADD CONSTRAINT call_sessions_status_check CHECK (status IN ('active', 'completed', 'cancelled', 'refunded'));
		EXCEPTION WHEN OTHERS THEN NULL;
		END $$;`,
		// 4. Reviews Table
		`CREATE TABLE IF NOT EXISTS reviews (
			id SERIAL PRIMARY KEY,
			session_id INT UNIQUE REFERENCES call_sessions(id),
			student_id INT REFERENCES users(id),
			teacher_id INT REFERENCES users(id),
			rating NUMERIC(3, 2) NOT NULL,
			comment TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		// 5. Wallet Transactions Table (bKash top-up tracking)
		`CREATE TABLE IF NOT EXISTS wallet_transactions (
			id SERIAL PRIMARY KEY,
			user_id INT REFERENCES users(id),
			payment_id VARCHAR(100) UNIQUE NOT NULL,
			trx_id VARCHAR(100),
			amount NUMERIC(10, 2) NOT NULL,
			status VARCHAR(20) CHECK (status IN ('initiated', 'completed', 'failed')) DEFAULT 'initiated',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		// 6. Withdrawal Requests Table (teacher requests payout, admin manually pays + marks done)
		`CREATE TABLE IF NOT EXISTS withdrawal_requests (
			id SERIAL PRIMARY KEY,
			teacher_id INT REFERENCES users(id),
			amount NUMERIC(10, 2) NOT NULL,
			payment_number VARCHAR(20) NOT NULL,
			method VARCHAR(20) DEFAULT 'bkash',
			status VARCHAR(20) CHECK (status IN ('pending', 'approved', 'rejected', 'paid')) DEFAULT 'pending',
			admin_note TEXT,
			requested_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			processed_at TIMESTAMP WITH TIME ZONE
		);`,

		// 7. Email Verification Codes
		`CREATE TABLE IF NOT EXISTS email_verifications (
			id SERIAL PRIMARY KEY,
			user_id INT REFERENCES users(id) ON DELETE CASCADE,
			code VARCHAR(10) NOT NULL,
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		// 8. Direct Messages (free messaging between any two users; calls are the paid part)
		`CREATE TABLE IF NOT EXISTS messages (
			id SERIAL PRIMARY KEY,
			sender_id INT REFERENCES users(id),
			receiver_id INT REFERENCES users(id),
			content TEXT NOT NULL,
			read_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_messages_participants ON messages (sender_id, receiver_id, created_at);`,
	}

	for _, query := range queries {
		_, err := DB.Exec(query)
		if err != nil {
			log.Fatalf("❌ Migration failed on query: %v", err)
		}
	}

	fmt.Println("✅ Database Tables Migrated Successfully!")
}
