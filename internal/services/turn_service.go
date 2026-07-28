package services

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"time"

	"varsity-network/internal/config"
)

// TurnCredentials is what we hand back to a logged-in client so it can add
// a TURN server to its ICE_SERVERS list.
type TurnCredentials struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username"`
	Credential string   `json:"credential"`
	TTL        int      `json:"ttl"`
}

// GenerateTurnCredentials implements coturn's standard "REST API" /
// time-limited shared-secret scheme (the same one used by Twilio, Metered.ca,
// etc): username = "<unix-expiry-timestamp>:<user-id>", credential =
// base64(HMAC-SHA1(secret, username)).
//
// This means:
//   - No permanent TURN username/password is ever embedded in the frontend
//     JS (which anyone could otherwise copy and abuse your bandwidth with).
//   - Credentials expire automatically (ttl seconds from now) — coturn
//     itself validates the HMAC and timestamp, no database lookup needed.
//
// Returns ok=false if TURN isn't configured yet (TURN_DOMAIN/TURN_SECRET
// unset), so the caller can fall back to STUN-only.
func GenerateTurnCredentials(cfg *config.Config, userID int) (creds TurnCredentials, ok bool) {
	if cfg.TurnDomain == "" || cfg.TurnSecret == "" {
		return TurnCredentials{}, false
	}

	ttl := 3600 // 1 hour — plenty for a single call session
	expiry := time.Now().Add(time.Duration(ttl) * time.Second).Unix()
	username := fmt.Sprintf("%d:user%d", expiry, userID)

	mac := hmac.New(sha1.New, []byte(cfg.TurnSecret))
	mac.Write([]byte(username))
	credential := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return TurnCredentials{
		URLs: []string{
			"turn:" + cfg.TurnDomain + ":3478?transport=udp",
			"turn:" + cfg.TurnDomain + ":3478?transport=tcp",
			"turns:" + cfg.TurnDomain + ":5349?transport=tcp",
		},
		Username:   username,
		Credential: credential,
		TTL:        ttl,
	}, true
}
