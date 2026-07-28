package models

import "time"

type ReviewRequest struct {
	SessionID int     `json:"session_id"`
	TeacherID int     `json:"teacher_id"`
	Rating    float64 `json:"rating"` // e.g., 1.0 to 5.0
	Comment   string  `json:"comment"`
}

type ReviewResponse struct {
	ID        int       `json:"id"`
	StudentID int       `json:"student_id"`
	TeacherID int       `json:"teacher_id"`
	Rating    float64   `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}
