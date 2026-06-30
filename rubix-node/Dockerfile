# ── Stage 1: Clone and build rubixgoplatform ─────────────────────────────────
#
# rubixgoplatform requires CGO (mattn/go-sqlite3) so we can't use scratch.
# The branch/tag is pinned via RUBIX_BRANCH; update it when you want to
# pick up a new release (e.g. --build-arg RUBIX_BRANCH=release-v1.1).
ARG RUBIX_BRANCH=release-v1
ARG GO_VERSION=1.22

FROM golang:${GO_VERSION}-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    git gcc g++ make ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ARG RUBIX_BRANCH
RUN git clone --depth=1 --branch "${RUBIX_BRANCH}" \
    https://github.com/rubixchain/rubixgoplatform.git /src

WORKDIR /src
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -trimpath -o /rubixgoplatform .

# ── Stage 2: Download Kubo (IPFS) ─────────────────────────────────────────────
FROM debian:bookworm-slim AS kubo-download
ARG KUBO_VERSION=v0.32.1

RUN apt-get update && apt-get install -y --no-install-recommends curl tar ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL \
    "https://dist.ipfs.tech/kubo/${KUBO_VERSION}/kubo_${KUBO_VERSION}_linux-amd64.tar.gz" \
    -o /tmp/kubo.tar.gz \
    && tar -xf /tmp/kubo.tar.gz -C /tmp \
    && mv /tmp/kubo/ipfs /usr/local/bin/ipfs \
    && rm -rf /tmp/kubo.tar.gz /tmp/kubo

# ── Stage 3: Runtime image ───────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libgcc-s1 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder       /rubixgoplatform     /app/rubixgoplatform
COPY --from=kubo-download /usr/local/bin/ipfs  /usr/local/bin/ipfs
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh /app/rubixgoplatform /usr/local/bin/ipfs

WORKDIR /app

# ── Ports ────────────────────────────────────────────────────────────────────
# 20000 — Node HTTP API  (internal: platform → node)
# 21000 — P2P sender     (must be reachable from the internet)
# 22000 — P2P receiver   (must be reachable from the internet)
# 4002  — IPFS swarm     (must be reachable from the internet)
# 5002  — IPFS API       (internal only)
EXPOSE 20000 21000 22000 4002 5002

ENTRYPOINT ["/app/entrypoint.sh"]
