package models

type TeacherProfileResponse struct {
	UserID         int     `json:"user_id"`
	Name           string  `json:"name"`
	Gender         string  `json:"gender"`
	University     string  `json:"university"`
	Expertise      string  `json:"expertise"`
	Bio            string  `json:"bio"`
	GirlsOnlyMode  bool    `json:"girls_only_mode"`
	TotalServices  int     `json:"total_services_given"`
	AverageRating  float64 `json:"average_rating"`
	IsOnline       bool    `json:"is_online"`
	KYCStatus      string  `json:"kyc_status"`
	StudentCardURL string  `json:"student_card_url,omitempty"`
}

type KYCApproveRequest struct {
	TeacherUserID int    `json:"teacher_user_id"`
	Status        string `json:"status"` // "approved" or "rejected"
}

type ToggleGirlsOnlyRequest struct {
	Enabled bool `json:"enabled"`
}

