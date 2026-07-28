package models

import "time"

type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Password  string    `json:"password,omitempty"`
	Role      string    `json:"role"` // student, teacher, admin
	Gender    string    `json:"gender"`
	Balance   float64   `json:"balance"`
	IsOnline  bool      `json:"is_online"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Role     string `json:"role"` // "student" or "teacher"
	Gender   string `json:"gender"`
	// Teacher-specific fields
	University     string `json:"university,omitempty"`
	Expertise      string `json:"expertise,omitempty"`
	StudentCardURL string `json:"student_card_url,omitempty"`
	// "I am not a robot" check
	CaptchaID     string `json:"captcha_id"`
	CaptchaAnswer int    `json:"captcha_answer"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Status  string `json:"status"`
	Token   string `json:"token"`
	Role    string `json:"role"`
	Message string `json:"message"`
}

// MeResponse is returned by GET /api/me — the logged-in user's own profile.
type MeResponse struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	Email           string  `json:"email"`
	Phone           string  `json:"phone"`
	Role            string  `json:"role"`
	Gender          string  `json:"gender"`
	Balance         float64 `json:"balance"`
	IsOnline        bool    `json:"is_online"`
	EmailVerified   bool    `json:"email_verified"`
	ProfilePhotoURL string  `json:"profile_photo_url"`
	CoverPhotoURL   string  `json:"cover_photo_url"`
	Headline        string  `json:"headline"`
	About           string  `json:"about"`
	Location        string  `json:"location"`
	Languages       string  `json:"languages"`

	// Populated only when Role == "teacher"
	University         string  `json:"university,omitempty"`
	Expertise          string  `json:"expertise,omitempty"`
	Bio                string  `json:"bio,omitempty"`
	KYCStatus          string  `json:"kyc_status,omitempty"`
	GirlsOnlyMode      bool    `json:"girls_only_mode,omitempty"`
	TotalServicesGiven int     `json:"total_services_given,omitempty"`
	AverageRating      float64 `json:"average_rating,omitempty"`
}

// PublicProfileResponse is what GET /api/profiles/{id} returns — safe to
// show to anyone (no email, phone, or balance).
type PublicProfileResponse struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	Role            string  `json:"role"`
	ProfilePhotoURL string  `json:"profile_photo_url"`
	CoverPhotoURL   string  `json:"cover_photo_url"`
	Headline        string  `json:"headline"`
	About           string  `json:"about"`
	Location        string  `json:"location"`
	Languages       string  `json:"languages"`
	IsOnline        bool    `json:"is_online"`
	JoinedAt        string  `json:"joined_at"`

	// Populated only when Role == "teacher"
	University         string  `json:"university,omitempty"`
	Expertise          string  `json:"expertise,omitempty"`
	Bio                string  `json:"bio,omitempty"`
	GirlsOnlyMode      bool    `json:"girls_only_mode,omitempty"`
	TotalServicesGiven int     `json:"total_services_given,omitempty"`
	AverageRating      float64 `json:"average_rating,omitempty"`
	KYCApproved        bool    `json:"kyc_approved,omitempty"`
}

// UpdateProfileRequest is used by PATCH /api/me to edit profile fields.
type UpdateProfileRequest struct {
	Headline        *string `json:"headline"`
	About           *string `json:"about"`
	Location        *string `json:"location"`
	Languages       *string `json:"languages"`
	ProfilePhotoURL *string `json:"profile_photo_url"`
	CoverPhotoURL   *string `json:"cover_photo_url"`

	// Teacher-only fields
	Bio        *string `json:"bio"`
	Expertise  *string `json:"expertise"`
	University *string `json:"university"`
}
