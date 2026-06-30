#!/usr/bin/env bash
# Runs inside the postgres container on first initialisation.
# POSTGRES_DB (platformdb) is already created by the entrypoint.
# This script creates the Rubix node database.
set -e

RUBIX_DB="${RUBIX_DB:-rubixdb}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    SELECT 'CREATE DATABASE $RUBIX_DB'
    WHERE NOT EXISTS (
        SELECT FROM pg_database WHERE datname = '$RUBIX_DB'
    )\gexec

    GRANT ALL PRIVILEGES ON DATABASE $RUBIX_DB TO $POSTGRES_USER;
EOSQL

echo "postgres init: created database '$RUBIX_DB'"
