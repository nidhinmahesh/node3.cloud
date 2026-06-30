# node3.cloud — Deployment Guide

This document is the single authoritative reference for deploying the node3.cloud stack on a fresh VM. Follow every step in order. Do not skip or reorder steps — several steps have hard dependencies on prior ones (TLS cert before Docker start, DNS before TLS cert, etc.).

---

## Stack overview

Four Docker Compose services, all on a private bridge network:

| Service | Image | Exposes externally |
|---|---|---|
| `postgres` | postgres:16-alpine | nothing |
| `node0` | built locally (clones rubixgoplatform from GitHub) | 21000, 22000, 4002 |
| `platform` | built locally (Go scratch image) | nothing |
| `nginx` | built locally (nginx + embedded SvelteKit static portal) | 80, 443 |

nginx is the only service reachable from the public internet on HTTP/HTTPS. The Rubix P2P ports (21000, 22000, 4002) are exposed directly for network participation.

---

## 1. VM requirements

- **OS:** Ubuntu 22.04 LTS (recommended) or Debian 12
- **CPU/RAM:** 4 vCPU / 8 GB RAM minimum
- **Disk:** 40 GB minimum (IPFS data grows over time)
- **Public IP:** static IP address

---

## 2. Firewall / security group

Open these ports **before** continuing. If your cloud provider uses a security group, configure it there. Otherwise use `ufw`:

```bash
ufw allow 22      # SSH — do this first or you will lock yourself out
ufw allow 80      # HTTP — required for ACME challenge and HTTP→HTTPS redirect
ufw allow 443     # HTTPS
ufw allow 21000   # Rubix P2P sender — must be reachable from the internet
ufw allow 22000   # Rubix P2P receiver — must be reachable from the internet
ufw allow 4002    # IPFS swarm — must be reachable from the internet
ufw enable
ufw status
```

**Do not expose port 20000** (rubix node HTTP API) — it is internal only and accessed by the platform container via Docker networking.

---

## 3. Install Docker and certbot

```bash
apt-get update
curl -fsSL https://get.docker.com | sh
apt-get install -y certbot
```

Verify:
```bash
docker --version          # should print Docker version 24+
docker compose version    # should print Docker Compose version 2+
certbot --version         # should print certbot 1.x or 2.x
```

---

## 4. DNS — point domain to this VM

**Do this before getting the TLS certificate.** certbot's ACME challenge requires the domain to resolve to the VM's IP.

Add two A records in your DNS provider:

```
node3.cloud      A  <VM-PUBLIC-IP>
www.node3.cloud  A  <VM-PUBLIC-IP>
```

Wait for propagation (usually 1–5 minutes), then verify:

```bash
dig +short node3.cloud
dig +short www.node3.cloud
```

Both must return your VM's IP before proceeding to step 5.

---

## 5. Get TLS certificate — BEFORE starting Docker

**This step must happen before `docker compose up`.** If Docker is already running and nginx is bound to port 80, certbot standalone will fail. Run certbot now while port 80 is free.

```bash
certbot certonly --standalone \
  -d node3.cloud \
  -d www.node3.cloud \
  --email nidhinmahesh1@gmail.com \
  --agree-tos \
  --no-eff-email
```

Expected output ends with:
```
Successfully received certificate.
Certificate is saved at: /etc/letsencrypt/live/node3.cloud/fullchain.pem
```

If this fails, the most common causes are:
- DNS not propagated yet (wait and retry)
- Port 80 blocked by firewall (check step 2)
- Wrong domain spelling

---

## 6. Clone the repository

```bash
git clone https://gitlab.com/fexr.club/bots/node3.cloud.git /opt/node3.cloud
cd /opt/node3.cloud
```

All subsequent commands in this guide assume you are in `/opt/node3.cloud`.

---

## 7. Place the mainnet swarm key

The swarm key restricts the IPFS network to the private Rubix mainnet swarm. **It is never committed to the repository.**

```bash
# Copy your swarm.key from wherever you have stored it
cp /path/to/your/swarm.key /opt/node3.cloud/rubix-node/swarm.key

# Confirm it is present and non-empty
ls -la rubix-node/swarm.key
head -1 rubix-node/swarm.key   # should print: /key/swarm/psk/1.0.0/
```

If you run without a swarm.key the node will join the public IPFS network instead of the Rubix mainnet swarm. The entrypoint prints a warning in that case.

---

## 8. Configure environment variables

```bash
cp .env.example .env
```

Generate random secrets (run each command separately and paste the output into `.env`):

```bash
openssl rand -hex 32   # use for POSTGRES_PASSWORD
openssl rand -hex 32   # use for SERVER_SECRET
openssl rand -hex 32   # use for CALLBACK_SECRET
```

### 8a. Telegram bot setup

1. Open Telegram, message [@BotFather](https://t.me/BotFather)
2. Send `/newbot` — follow prompts, copy the token → `TELEGRAM_BOT_TOKEN`
3. Send `/setdomain` → select your bot → type `node3.cloud`
   **This step is required** — without it the Telegram Login Widget will refuse to work on your domain.
4. The bot's username (without `@`) → `VITE_TELEGRAM_BOT`

### 8b. Lemon Squeezy setup

1. Create account at [lemonsqueezy.com](https://lemonsqueezy.com)
2. Create a Store, then create a **Subscription** product for the Pro plan
3. Note the **Variant ID** of that product → `LEMON_VARIANT_ID`
4. Dashboard → Settings → API Keys → create key → `LEMON_API_KEY`
5. Dashboard → Settings → Webhooks → Add webhook:
   - **URL:** `https://node3.cloud/api/lemon/webhook`
     ⚠️ The URL is `/api/lemon/webhook`, not `/api/billing/webhook`
   - **Events to enable:** `subscription_created`, `subscription_updated`, `subscription_cancelled`, `subscription_expired`, `subscription_payment_failed`
   - Copy the **Signing Secret** → `LEMON_WEBHOOK_SECRET`

### 8c. Final .env file

After filling in all values, `.env` should look like this (no blank values):

```env
POSTGRES_USER=node3
POSTGRES_PASSWORD=<32-byte-hex>
POSTGRES_DB=platformdb
RUBIX_DB=rubixdb

SERVER_SECRET=<32-byte-hex>
CALLBACK_SECRET=<32-byte-hex>

TELEGRAM_BOT_TOKEN=<from BotFather>
LEMON_WEBHOOK_SECRET=<from Lemon Squeezy webhook page>
LEMON_API_KEY=<from Lemon Squeezy API keys page>
LEMON_VARIANT_ID=<numeric variant ID>

PLATFORM_URL=https://node3.cloud

VITE_TELEGRAM_BOT=<bot username without @>

RUBIX_BRANCH=release-v1
```

**`.env` is in `.gitignore`. Never commit it.**

---

## 9. Build Docker images

This step clones `rubixgoplatform` from GitHub inside the builder container and compiles it with CGO. It takes 3–8 minutes depending on VM speed.

```bash
docker compose build
```

Expected: all four services build without errors. The `node0` build will print git clone output followed by Go compilation output.

If `node0` build fails, the most common cause is a temporary GitHub rate limit — wait 60 seconds and retry.

---

## 10. Start the stack

```bash
docker compose up -d
```

Watch logs until all services are stable:

```bash
docker compose logs -f
```

What to expect:
- `postgres` starts in ~5 seconds, prints `database system is ready to accept connections`
- `node0` starts after postgres is healthy; IPFS initialises its repo on first run (~10 s), then rubixgoplatform starts
- `platform` starts after postgres is healthy; prints `node3.cloud platform listening on :8080` and runs the schema migration automatically
- `nginx` starts last; because TLS certs exist (from step 5) it starts in full HTTPS mode immediately

Press Ctrl-C to stop following logs. The stack continues running in the background.

---

## 11. Verify the stack

Run these checks in order. Each must pass before continuing.

```bash
# All four containers are running (not restarting)
docker compose ps

# Postgres is healthy
docker compose exec postgres pg_isready -U node3 -d platformdb

# Platform schema was applied (platform tables exist)
docker compose exec postgres psql -U node3 -d platformdb \
  -c "\dt platform.*"
# Should list: accounts, account_usage, api_keys, hosted_contracts, ...

# Platform can reach the rubix node
docker compose exec platform wget -qO- http://node0:20000/rubix/v1/node/status
# Should return JSON, not a connection error

# HTTPS is working
curl -I https://node3.cloud
# Should return HTTP/2 200

# Platform API is reachable
curl -s https://node3.cloud/api/auth/me
# Should return {"error":"missing token"} — 401 is correct, it means the API is live

# Portal loads
curl -sI https://node3.cloud | grep content-type
# Should contain text/html
```

If `platform` can't reach `node0`, check that node0 is running and that `RUBIX_NODE_HOST=node0` is set in the platform environment:
```bash
docker compose exec platform env | grep RUBIX_NODE_HOST
```

---

## 12. Apply the Postgres index for webhook performance

The webhook worker polls `fullnode_transactions` by `created_at`. This index is needed for efficient polling as the table grows. Apply it once the rubix node has been running for at least 60 seconds (it creates the table on first start).

```bash
docker compose exec postgres psql -U node3 -d rubixdb -c "
  CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_fullnode_txn_created_at
  ON fullnode_transactions (created_at);
"
```

Expected output: `CREATE INDEX`

If it prints `ERROR: table "fullnode_transactions" does not exist`, wait 60 seconds and retry — the rubix node creates the table on first use.

---

## 13. Test end-to-end with a user account

1. Open `https://node3.cloud` in a browser
2. The landing page loads and the Telegram Login Widget appears
3. Click "Login with Telegram" — the Telegram widget opens
4. Authenticate — you are redirected to `/setup`
5. Generate and back up the 12-word mnemonic, set a PIN
6. A DID is created — you are redirected to `/dashboard`
7. On the dashboard, create an API key
8. Test the API key:
   ```bash
   curl -H "X-API-Key: <your-key>" https://node3.cloud/rubix/v1/node/status
   ```
   Should return node status JSON.

---

## 14. Certificate auto-renewal

Let's Encrypt certificates expire after 90 days. Set up automatic renewal using the webroot method (zero nginx downtime). The `certbot_www` Docker volume is already mounted in nginx at `/var/www/certbot` for this purpose.

```bash
# Get the volume's path on the host filesystem
WEBROOT=$(docker volume inspect node3cloud_certbot_www --format '{{.Mountpoint}}')
echo "Webroot: $WEBROOT"   # confirm a path was printed
```

⚠️ The volume name prefix (`node3cloud_`) comes from the Docker Compose project name, which defaults to the directory name. If you cloned to `/opt/node3.cloud`, the project name is `node3cloud`. If you used a different directory, adjust accordingly.

```bash
# Add a daily renewal attempt (no-ops if cert is not due yet)
(crontab -l 2>/dev/null; echo "0 3 * * * certbot renew --webroot -w $WEBROOT --quiet --deploy-hook 'docker compose -f /opt/node3.cloud/docker-compose.yml exec -T nginx nginx -s reload'") | crontab -

# Verify it was added
crontab -l
```

Test the renewal process (dry run, does not actually renew):
```bash
certbot renew --webroot -w $WEBROOT --dry-run
```

---

## 15. Upgrading the rubix node binary

When a new `rubixgoplatform` release is tagged on GitHub:

```bash
cd /opt/node3.cloud

# Build only the node0 image with the new branch/tag
docker compose build --build-arg RUBIX_BRANCH=release-v1.1 node0

# Restart node0 (postgres, platform, nginx keep running with zero downtime)
docker compose up -d node0

# Watch node0 restart
docker compose logs -f node0
```

---

## 16. Upgrading the platform or portal

```bash
cd /opt/node3.cloud
git pull

# Rebuild only what changed
docker compose build platform    # if Go code changed
docker compose build nginx       # if portal or nginx config changed

docker compose up -d             # rolling restart of changed services
```

---

## 17. Adding a dedicated node for a paid user (future)

When the `platform.nodes` pool is empty, paid users fall back to the shared `node0`. To add a dedicated node:

1. Spin up an additional rubixgoplatform process (node index 1, port 20001) — use the `rubix-node@.service` systemd template in `infra/systemd/` as reference
2. Insert a row into the pool:
   ```bash
   docker compose exec postgres psql -U node3 -d platformdb -c "
     INSERT INTO platform.nodes (id, port, status) VALUES (1, 20001, 'available');
   "
   ```
3. The next paid subscription upgrade will automatically claim this node.

---

## Common failure modes

| Symptom | Cause | Fix |
|---|---|---|
| `nginx` keeps restarting | TLS cert path wrong or missing | Run step 5, verify `/etc/letsencrypt/live/node3.cloud/fullchain.pem` exists |
| `platform` exits immediately | Missing required env var | `docker compose logs platform` — it prints which var is missing |
| `node0` can't reach postgres | Postgres still initialising | Wait 30 s, `docker compose restart node0` |
| Telegram widget says "Bot domain invalid" | `/setdomain` not set | Message @BotFather, `/setdomain`, set to `node3.cloud` |
| Lemon Squeezy webhook 404 | Wrong webhook URL registered | URL must be `https://node3.cloud/api/lemon/webhook` |
| `curl https://node3.cloud` returns connection refused | nginx not running | `docker compose ps nginx`, check logs |
| DID creation fails with "node unreachable" | node0 not ready yet | Wait 60 s for IPFS peer discovery, retry |
| `fullnode_transactions` doesn't exist | Rubix node hasn't started yet | Wait 60 s after `docker compose up`, retry step 12 |
