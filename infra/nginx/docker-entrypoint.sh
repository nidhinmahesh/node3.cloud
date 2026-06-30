#!/usr/bin/env sh
# nginx entrypoint: start HTTP-only if TLS certs are not yet present,
# then reload when certs appear (e.g. after certbot runs).
set -e

CERT="/etc/letsencrypt/live/node3.cloud/fullchain.pem"
HTTP_CONF="/etc/nginx/conf.d/http-only.conf"
FULL_CONF="/etc/nginx/conf.d/default.conf"

if [ ! -f "$CERT" ]; then
    echo "[nginx] TLS cert not found at $CERT"
    echo "[nginx] Starting in HTTP-only mode for ACME challenge"
    echo "[nginx] Run: certbot certonly --webroot -w /var/www/certbot -d node3.cloud -d www.node3.cloud"
    echo "[nginx] Then restart this container to enable HTTPS"

    # Temporarily replace the full HTTPS config with an HTTP-only stub.
    cat > "$HTTP_CONF" <<'EOF'
server {
    listen 80;
    server_name node3.cloud www.node3.cloud;

    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    location / {
        proxy_pass http://platform:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
EOF
    # Disable the full config that needs certs.
    mv "$FULL_CONF" "${FULL_CONF}.disabled" 2>/dev/null || true
fi

exec nginx -g "daemon off;"
