package services

import (
	"database/sql"
	"log"
	"time"

	"varsity-network/internal/database"
	ws "varsity-network/internal/websocket"
)

// MarkSessionConnected flags that the call actually connected (the callee
// answered). Called from the WebSocket handler when it relays an "answer"
// signal. Safe to call multiple times.
func MarkSessionConnected(sessionID int) {
	_, err := database.DB.Exec(`UPDATE call_sessions SET is_connected = true WHERE id = $1 AND status = 'active'`, sessionID)
	if err != nil {
		log.Printf("⚠️ Failed to mark session %d as connected: %v", sessionID, err)
	}
}

// FinalizeCallSession settles the money for a call session exactly once.
// This is the single source of truth for "who gets paid" — it is called
// from three places: the auto-disconnect timer (endedBy="timeout"), and the
// manual end-call HTTP endpoint (endedBy="student" or "teacher", derived
// from whichever participant actually clicked "end call").
//
// Rules:
//   - The call never actually connected (the other side never answered) ->
//     always refund the student in full, regardless of who "ended" it.
//     Nobody should be charged for a call that never happened.
//   - endedBy == "teacher" (call did connect): the teacher hung up early ->
//     the student is refunded in full, the teacher earns nothing.
//   - endedBy == "student" or "timeout" (call did connect): the student
//     ended it early (their choice) or the package time simply ran out ->
//     the teacher earns the full package amount, and their
//     total_services_given count goes up.
//
// It is idempotent: if the session is no longer "active" (already settled
// by a previous call), it does nothing and returns nil.
func FinalizeCallSession(hub *ws.Hub, sessionID, studentID, teacherID int, endedBy string) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}

	var status string
	var amount float64
	var isConnected bool
	err = tx.QueryRow(`SELECT status, amount, is_connected FROM call_sessions WHERE id = $1 FOR UPDATE`, sessionID).Scan(&status, &amount, &isConnected)
	if err == sql.ErrNoRows {
		tx.Rollback()
		return nil
	} else if err != nil {
		tx.Rollback()
		return err
	}

	if status != "active" {
		// Already settled by an earlier call to this function — nothing to do.
		tx.Rollback()
		return nil
	}

	if !isConnected {
		// The call never actually connected (teacher never answered, or the
		// student gave up while it was still ringing) — always refund, no
		// matter who technically clicked "end". Nobody rendered a service.
		if _, err = tx.Exec(
			`UPDATE call_sessions SET status = 'cancelled', ended_by = $1, ended_at = CURRENT_TIMESTAMP WHERE id = $2`,
			endedBy, sessionID,
		); err != nil {
			tx.Rollback()
			return err
		}
		if _, err = tx.Exec(`UPDATE users SET balance = balance + $1 WHERE id = $2`, amount, studentID); err != nil {
			tx.Rollback()
			return err
		}
		log.Printf("↩️ Session %d cancelled + refunded to student %d (never connected)", sessionID, studentID)
	} else if endedBy == "teacher" {
		if _, err = tx.Exec(
			`UPDATE call_sessions SET status = 'refunded', ended_by = $1, ended_at = CURRENT_TIMESTAMP WHERE id = $2`,
			endedBy, sessionID,
		); err != nil {
			tx.Rollback()
			return err
		}
		if _, err = tx.Exec(`UPDATE users SET balance = balance + $1 WHERE id = $2`, amount, studentID); err != nil {
			tx.Rollback()
			return err
		}
		log.Printf("💸 Session %d refunded to student %d (teacher ended early)", sessionID, studentID)
	} else {
		if _, err = tx.Exec(
			`UPDATE call_sessions SET status = 'completed', ended_by = $1, ended_at = CURRENT_TIMESTAMP WHERE id = $2`,
			endedBy, sessionID,
		); err != nil {
			tx.Rollback()
			return err
		}
		if _, err = tx.Exec(`UPDATE users SET balance = balance + $1 WHERE id = $2`, amount, teacherID); err != nil {
			tx.Rollback()
			return err
		}
		if _, err = tx.Exec(`UPDATE teacher_profiles SET total_services_given = total_services_given + 1 WHERE user_id = $1`, teacherID); err != nil {
			tx.Rollback()
			return err
		}
		log.Printf("💰 Session %d paid out to teacher %d (%.2f, ended_by=%s)", sessionID, teacherID, amount, endedBy)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if hub != nil {
		msg := ws.SignalMessage{
			Type:     "call_ended",
			SenderID: 0, // server
			Data:     map[string]interface{}{"session_id": sessionID, "ended_by": endedBy},
		}
		msg.TargetID = studentID
		hub.Broadcast <- msg
		msg.TargetID = teacherID
		hub.Broadcast <- msg
	}

	return nil
}

// StartCallTimer auto-ends and settles the call once the purchased package
// minutes run out, in case neither side manually hangs up.
func StartCallTimer(hub *ws.Hub, sessionID, studentID, teacherID, packageMinutes int) {
	duration := time.Duration(packageMinutes) * time.Minute
	log.Printf("⏱️ Call Timer started for Session %d: %d minutes", sessionID, packageMinutes)

	go func() {
		time.Sleep(duration)
		if err := FinalizeCallSession(hub, sessionID, studentID, teacherID, "timeout"); err != nil {
			log.Printf("❌ Failed to finalize session %d on timeout: %v", sessionID, err)
		}
	}()
}
