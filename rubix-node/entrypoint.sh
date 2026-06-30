#!/usr/bin/env bash
# Entrypoint for the rubixgoplatform container.
# Environment variables (all required unless defaulted):
#   NODE_INDEX   — node index; determines port (20000+N) and data dir (nodeN/)
#   DB_HOST      — postgres host (set to "postgres" in docker-compose)
#   DB_PORT      — postgres port (default 5432)
#   DB_USER      — postgres user
#   DB_PASSWORD  — postgres password
#   DB_NAME      — rubix node database name
#   SWARM_KEY    — path to swarm.key file (default /run/secrets/swarm_key)
set -e

NODE_INDEX="${NODE_INDEX:-0}"
NODE_DIR="node${NODE_INDEX}"
DB_HOST="${DB_HOST:-postgres}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-node3}"
DB_PASSWORD="${DB_PASSWORD:?DB_PASSWORD is required}"
DB_NAME="${DB_NAME:-rubixdb}"
SWARM_KEY_PATH="${SWARM_KEY:-/run/secrets/swarm_key}"

echo "[rubix-node] starting node${NODE_INDEX} (port $((20000 + NODE_INDEX)))"

mkdir -p "/app/${NODE_DIR}"

# If a swarm.key file is mounted, copy it to the node data dir so IPFS picks
# it up when it initialises its repo (IPFS looks for swarm.key in $IPFS_PATH).
# rubixgoplatform stores IPFS data inside the node directory.
if [ -f "${SWARM_KEY_PATH}" ]; then
    IPFS_REPO="/app/${NODE_DIR}/.ipfs"
    mkdir -p "${IPFS_REPO}"
    cp "${SWARM_KEY_PATH}" "${IPFS_REPO}/swarm.key"
    echo "[rubix-node] swarm.key installed at ${IPFS_REPO}/swarm.key"
else
    echo "[rubix-node] WARNING: no swarm.key found at ${SWARM_KEY_PATH} — node will join the public IPFS network"
fi

exec /app/rubixgoplatform run \
    -p "${NODE_DIR}" \
    -n "${NODE_INDEX}" \
    -s \
    -fullnode \
    -dbAddress "${DB_HOST}" \
    -dbPort    "${DB_PORT}" \
    -dbUsername "${DB_USER}" \
    -dbPassword "${DB_PASSWORD}" \
    -dbName    "${DB_NAME}"
