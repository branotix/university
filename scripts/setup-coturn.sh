#!/bin/bash
# ==============================================================================
# Varsity Network — Production TURN server setup (coturn)
# ==============================================================================
# Run this as root on a FRESH Ubuntu 22.04/24.04 VPS that has its own public
# IP address and a domain name pointed at it (an A record: turn.yourdomain.com
# -> your VPS IP). This is a SEPARATE machine from your app server — coturn
# can live on the same box as the Go backend too, just make sure the ports
# below don't conflict with anything else you're running.
#
# USAGE:
#   chmod +x setup-coturn.sh
#   sudo ./setup-coturn.sh turn.yourdomain.com "a-long-random-secret-string"
#
# The secret you pass in must match TURN_SECRET in your Go backend's .env —
# this is what lets the backend hand out short-lived, per-user TURN
# credentials (see internal/services/turn_service.go) instead of one
# permanent password anyone could copy out of the frontend JS.
# ==============================================================================

set -e

DOMAIN="$1"
SECRET="$2"

if [ -z "$DOMAIN" ] || [ -z "$SECRET" ]; then
  echo "Usage: sudo ./setup-coturn.sh turn.yourdomain.com \"your-shared-secret\""
  echo "Generate a good secret with: openssl rand -hex 32"
  exit 1
fi

echo "=== 1. Installing coturn + certbot ==="
apt update
apt install -y coturn certbot

echo "=== 2. Enabling the coturn service ==="
sed -i 's/#TURNSERVER_ENABLED=1/TURNSERVER_ENABLED=1/' /etc/default/coturn

echo "=== 3. Getting a TLS certificate for $DOMAIN ==="
echo "    (make sure the domain's DNS A record already points at this server's IP!)"
certbot certonly --standalone --non-interactive --agree-tos \
  -m "admin@$DOMAIN" -d "$DOMAIN" || {
    echo "Certbot failed — check that $DOMAIN points at this server's public IP and that port 80 is free."
    exit 1
  }

echo "=== 4. Writing /etc/turnserver.conf ==="
cat > /etc/turnserver.conf << EOF
# Basic listening setup
listening-port=3478
tls-listening-port=5349
min-port=49152
max-port=65535

# Auth: time-limited shared-secret (coturn's "REST API" scheme).
# The Go backend generates per-user, expiring credentials with this same
# secret — see internal/services/turn_service.go
use-auth-secret
static-auth-secret=$SECRET
realm=$DOMAIN

# TLS (for turns:// — works through more restrictive firewalls than plain turn://)
cert=/etc/letsencrypt/live/$DOMAIN/fullchain.pem
pkey=/etc/letsencrypt/live/$DOMAIN/privkey.pem

# Hardening
fingerprint
no-multicast-peers
no-cli
stale-nonce=600
# Cap how much bandwidth any single relayed session can use (bytes/sec).
# 1000000 = ~1 Mbps per relayed stream direction; tune to your bandwidth budget.
max-bps=1000000
# Cap total simultaneous relay sessions — protects your server from being overwhelmed.
total-quota=200
EOF

echo "=== 5. Opening firewall ports (ufw) ==="
if command -v ufw >/dev/null; then
  ufw allow 3478/udp
  ufw allow 3478/tcp
  ufw allow 5349/udp
  ufw allow 5349/tcp
  ufw allow 49152:65535/udp
else
  echo "ufw not found — make sure these ports are open in your cloud provider's firewall/security group:"
  echo "  3478/udp, 3478/tcp, 5349/udp, 5349/tcp, 49152-65535/udp"
fi

echo "=== 6. Starting coturn ==="
systemctl restart coturn
systemctl enable coturn

echo "=== 7. Setting up cert auto-renewal ==="
cat > /etc/cron.d/coturn-cert-renew << 'CRON'
0 3 * * * root certbot renew --quiet --deploy-hook "systemctl restart coturn"
CRON

echo ""
echo "✅ Done! Now add this to your Go backend's .env:"
echo ""
echo "   TURN_DOMAIN=$DOMAIN"
echo "   TURN_SECRET=$SECRET"
echo ""
echo "Then restart the Go server. Test with: https://webrtc.github.io/samples/src/content/peerconnection/trickle-ice/"
echo "(paste in a turn: URL using the credentials your backend's GET /api/turn-credentials returns)"
