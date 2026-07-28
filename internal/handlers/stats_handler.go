package handlers

import (
	"encoding/json"
	"net/http"

	"varsity-network/internal/database"
)

// GetPlatformStatsHandler returns public, aggregate numbers for the feed
// page: how many students/teachers are online right now, and how many
// sessions have been completed on the platform so far.
func GetPlatformStatsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var onlineStudents, onlineTeachers, totalCompletedSessions int

		database.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'student' AND is_online = true`).Scan(&onlineStudents)
		database.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'teacher' AND is_online = true`).Scan(&onlineTeachers)
		database.DB.QueryRow(`SELECT COUNT(*) FROM call_sessions WHERE status = 'completed'`).Scan(&totalCompletedSessions)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data": map[string]int{
				"online_students":          onlineStudents,
				"online_teachers":          onlineTeachers,
				"online_total":             onlineStudents + onlineTeachers,
				"total_completed_sessions": totalCompletedSessions,
			},
		})
	}
}
