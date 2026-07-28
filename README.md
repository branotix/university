# Varsity Network

A platform connecting students with verified university seniors (BUET, DU, DUET etc.)
for 1:1 audio/video mentorship calls, with wallet top-up via bKash and
university-based teacher discovery.

## Latest update: captcha + rate limiting, live stats, Terms & Conditions, production TURN server

- **"I am not a robot" check**: a lightweight, self-hosted math captcha
  (`GET /api/captcha`, e.g. "4 + 7 = ?") — no external API key needed (unlike
  reCAPTCHA). Required on both registration and "resend code".
- **Resend rate limit**: at most **one verification code per 60 seconds**
  per account, enforced server-side (checked against the last code's
  timestamp in the database) — not just a client-side cooldown, so it can't
  be bypassed. The resend button also shows a live countdown.
- **Live platform stats on the feed page**: how many students/teachers are
  online right now, and total completed sessions — powered by
  `GET /api/stats`, refreshes every 20 seconds.
- **Terms & Conditions** on registration: a required checkbox + a 7-point
  Bengali terms modal, explicitly including a zero-tolerance clause on
  sexual harassment (with a legal-action warning) and respect toward
  seniors. Registration is blocked until it's checked.
- **Production TURN server setup**: see `scripts/setup-coturn.sh` and the
  "Setting up a TURN server" section below. TURN credentials are no longer
  hardcoded anywhere — the backend generates fresh, per-user, 1-hour
  credentials on demand (`GET /api/turn-credentials`, coturn's standard
  time-limited shared-secret scheme), which `call.js` fetches automatically
  before every call.

## Setting up a TURN server (do this before real users rely on calling)

Without a TURN server, roughly 20-30% of real-world calls — mobile data,
restrictive campus/office wifi, some ISPs — won't be able to connect
peer-to-peer and will just hang on "connecting...". A TURN server relays the
call through itself in those cases.

1. Get a small VPS (1 vCPU / 1GB RAM is plenty to start) with its own public
   IP, and point a domain's A record at it (e.g. `turn.yourdomain.com`).
2. Generate a secret: `openssl rand -hex 32`
3. Copy `scripts/setup-coturn.sh` onto that VPS and run:
   ```
   chmod +x setup-coturn.sh
   sudo ./setup-coturn.sh turn.yourdomain.com "the-secret-you-generated"
   ```
   This installs and configures `coturn` with a real TLS certificate
   (auto-renewing), opens the right firewall ports, and sets up the
   time-limited shared-secret auth scheme.
4. Add to your Go backend's `.env`:
   ```
   TURN_DOMAIN=turn.yourdomain.com
   TURN_SECRET=the-same-secret-you-used-above
   ```
5. Restart the Go server. `call.js` will now automatically fetch and use
   TURN credentials for every call — no frontend changes needed.

You can verify it's working with the trickle-ICE test page linked at the end
of the setup script's output, using credentials from
`GET /api/turn-credentials` (call it while logged in, e.g. via your
browser's dev tools on the site).

**Bandwidth note**: relayed (TURN) calls use your VPS's bandwidth directly —
budget roughly 1-3 Mbps per relayed call pair. The setup script caps
per-session bandwidth (`max-bps`) and total concurrent sessions
(`total-quota`) in `/etc/turnserver.conf` — tune those to your server's
actual bandwidth plan.

## Latest update: bug fixes + profiles/messaging/multi-page redesign

- **Fixed: student charged even when the teacher never answered.** Added
  `is_connected` tracking on `call_sessions` (set the moment the callee's
  WebRTC answer comes through). If a call never actually connects, ending it
  — no matter who technically clicks "end" — now always refunds the student
  in full. The student/teacher "who gets paid" rule only kicks in for calls
  that did connect.
- **Chrome call-receiving issue:** the most likely cause is that
  `getUserMedia` (camera/mic access) requires a secure context — `https://`
  or exactly `localhost`. If you were testing over a plain `http://` LAN IP
  or a non-https tunnel, Chrome silently blocks it (Firefox can be more
  lenient in some local setups, which is why it "worked" there). `call.js`
  now shows a clear warning if you're on an insecure origin, and also
  handles Chrome's stricter autoplay policy for the remote video (with a
  "tap to enable" fallback if audio/video autoplay gets blocked). Test over
  `https://` (e.g. an ngrok tunnel) or `http://localhost`, not a raw IP.
- **KYC is now a real image upload** (`POST /api/upload?type=kyc`, 5MB max,
  jpg/png/webp) instead of a pasted URL.
- **Email verification** on registration — a 6-digit code is sent via Gmail
  SMTP (see setup below) and must be verified before login works. If SMTP
  isn't configured yet, the code is returned in the API response / printed
  to the server console as a dev convenience — **turn this off before real
  users sign up** (just set `SMTP_USER`/`SMTP_PASSWORD`).
- **Removed the "block student" feature** per your request — the endpoint,
  UI, and eligibility check have all been removed.
- **Full Wikipedia-style profiles**: headline, about, location, languages,
  profile photo, cover photo — editable after registration, with a
  completion-percentage indicator. Every user (student or teacher) has a
  public profile page anyone can visit.
- **Free messaging + paid calls from a profile page**: visit anyone's
  profile → "Message" (free, real conversation thread) or, for approved
  teachers, "Book a Call" (opens the same paid package flow as before).
- **Persistent login (~30 days)**: JWT expiry extended from 24 hours to 30
  days, and login now also sets an `HttpOnly` cookie (`vn_token`) in
  addition to the token returned in the response body — so you generally
  won't get logged out just from closing the browser.
- **Multi-page app shell** (Facebook/X-style) instead of one flat dashboard:
  `feed.html` (browse everyone's profile), `profile.html` (view/edit),
  `messages.html` (conversations), `settings.html` (Girls Only Mode,
  withdrawals, KYC status). A shared top nav (`js/nav.js`) ties them
  together, and `js/presence.js` keeps the WebSocket connection (and
  incoming-call popup for teachers) alive no matter which page you're on.
  `student.html`/`teacher.html` no longer exist — replaced by the above.

## What's in this build

**Backend (Go)** — your original backend, with these fixes/additions:
- Removed the committed `.env` (secrets were exposed) → replaced with `.env.example`
- Blocked self-registration as "admin" (was a security hole)
- Added real admin-role check on `/api/admin/approve-kyc` (previously anyone logged in could approve any teacher)
- Added `GET /api/me`, girls-only-mode toggle, block-student (moderation), and
  `GET /api/admin/pending-teachers` endpoints
- Added CORS middleware
- **Real bKash Tokenized Checkout sandbox integration** for wallet top-up
  (server verifies payment with bKash directly — never trusts a client-sent amount)
- Added eligibility checks to package purchase: Girls Only Mode + blocked-student list
- **Fixed: teacher never got paid.** Payment used to only happen when a
  student submitted a review — if they skipped it, the money just vanished
  (deducted from the student, never credited to the teacher). Payment is now
  settled by `services.FinalizeCallSession`, triggered the moment a call
  actually ends (manual hangup or package-time timeout), completely
  independent of whether a review is ever submitted.
- **Fixed: no distinction between who ended the call.** Added `ended_by`
  tracking on `call_sessions`. Business rule now enforced server-side:
  - Package time runs out, or the **student** ends the call early → teacher
    gets the full amount (matches your original spec).
  - The **teacher** ends the call early (or declines/cancels before
    connecting) → the **student is refunded in full**, teacher gets nothing
    for that session.
  - See `POST /api/sessions/end` (`internal/handlers/session_handler.go`) —
    it derives "who ended it" from the authenticated caller's own identity,
    never from a value the client could fake.
- **Added teacher withdrawal flow**, mirroring the KYC approval pattern:
  - `POST /api/teacher/withdraw` — teacher requests a payout to a bKash/Nagad
    number; the amount is held (deducted from their balance immediately so
    they can't double-request more than they have).
  - `GET /api/teacher/withdrawals` — teacher sees their own request history.
  - `GET /api/admin/withdrawals/pending` — admin sees pending requests.
  - `POST /api/admin/withdrawals/process` — admin marks a request "paid"
    (after manually sending the money via bKash/Nagad outside this system)
    or "rejected" (held amount is refunded back to the teacher).

**Frontend (`/web`, vanilla HTML/CSS/JS, served by the Go server itself)**
- `index.html` — login / registration
- `student.html` — browse teachers (university filter), buy a call package, top up wallet
- `teacher.html` — earnings, KYC status, Girls Only Mode toggle, block student,
  **withdrawal requests**, and **listens for incoming calls**
- `call.html` — the actual 1:1 video/audio call (WebRTC + WebSocket signaling)
- `review.html` — star rating after a call (rating only — no longer moves money)
- `admin.html` — approve/reject teacher KYC, and approve/reject withdrawal requests

## Setting up Gmail for email verification

1. On the Gmail account you want to send from: turn on 2-Step Verification
   (Google Account → Security).
2. Go to Google Account → Security → App passwords, create one for "Mail".
3. Put that 16-character password (not your normal Gmail password) in
   `SMTP_PASSWORD`, and the Gmail address in `SMTP_USER` (and `SMTP_FROM`).
4. Restart the server. New registrations will now get a real email.

Until you do this, verification codes are just printed to the server
console and returned in the API response (`dev_code`) so you can still test
locally — remove that fallback expectation before letting real users sign up.

## Running it locally

1. Copy `.env.example` to `.env` and fill in your real Postgres credentials
   (the bKash sandbox credentials in the example are bKash's public test
   credentials — fine to use as-is for testing).

2. Start Postgres, then run:
   ```
   go mod tidy
   go run ./cmd/api
   ```
   If you already had this database running before this update, the new
   `ended_by` column and `withdrawal_requests` table are added automatically
   on startup (`internal/database/db.go` runs safe `ADD COLUMN IF NOT EXISTS`
   / `CREATE TABLE IF NOT EXISTS` migrations) — no manual SQL needed.

3. Open http://localhost:8080 in your browser.

## Testing bKash top-up locally

bKash's sandbox needs to redirect back to a **publicly reachable** URL after
payment — it can't reach `http://localhost:8080` directly. For local testing,
use a tunnel like [ngrok](https://ngrok.com):
```
ngrok http 8080
```
Then set `PUBLIC_BASE_URL=https://your-ngrok-subdomain.ngrok-free.app` in your `.env`
and restart the server. Once you're ready to go live, switch `BKASH_BASE_URL`
to the production URL and use your real merchant credentials instead of the
sandbox ones.

## Creating an admin account

Registration deliberately blocks creating "admin" accounts through the API
(so a random visitor can't make themselves an admin). To create one:

1. Register a normal account (as a student) through the app with the email
   you want to use as admin.
2. Run this SQL directly against your database:
   ```sql
   UPDATE users SET role = 'admin' WHERE email = 'your-admin-email@example.com';
   ```
3. Log in with that account and visit `/admin.html` directly.

## Calling: how it works

- WebSocket (`/ws`) handles signaling only (who's calling whom, SDP offer/answer, ICE candidates).
- WebRTC handles the actual audio/video stream, peer-to-peer.
- STUN (Google's free public server) is used by default. This works for most
  connections, but roughly 20-30% of real-world calls — especially on mobile
  data or behind restrictive networks — will fail without a **TURN** server to
  relay media. Before going live, add a TURN server in `web/js/call.js`
  (`ICE_SERVERS` array) — see comments in that file. Free options to start
  with: a self-hosted `coturn` instance, or free tiers from Metered.ca / Twilio.
- The call auto-ends when the purchased package time runs out (handled
  server-side in `internal/services/call_service.go`), or when either side
  manually hangs up.

## Known limitations / things to do before real users touch this

- No file upload for student ID cards yet — registration expects a URL
  (e.g. upload the image somewhere and paste the link). Add real file upload
  (e.g. to S3 or similar) before launch.
- No rate limiting / brute-force protection on login.
- No password-reset flow.
- If a call is left "active" forever because neither side clicks "end call"
  nor closes the tab (e.g. a crashed browser that never fires `beforeunload`),
  the session is only settled when the purchased package time fully elapses
  (the server-side timer). This is a bounded worst case (at most
  `package_minutes`), not an indefinite hang, but you may want to also settle
  sessions when a WebSocket disconnects if you need faster resolution.
- If the call never actually connects (e.g. the teacher never answers) and
  the **student** manually clicks "end call" while waiting, your original
  spec's rule ("student ends early → teacher keeps full payment") still
  applies literally, even though no call happened. If that's not what you
  want, we can add a "cancel while ringing, before answer" path that refunds
  the student instead — let me know.
