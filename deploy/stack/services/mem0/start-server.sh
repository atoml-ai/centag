#!/bin/bash
set -euo pipefail

PG_DB="${APP_DB_NAME:-mem0_app}"
echo "Ensuring database ${PG_DB} exists..."
PGPASSWORD="${POSTGRES_PASSWORD:-postgres}" psql \
    -h "${POSTGRES_HOST:-postgres}" -p "${POSTGRES_PORT:-5432}" \
    -U "${POSTGRES_USER:-postgres}" -d postgres \
    -c "CREATE DATABASE \"${PG_DB}\";" 2>/dev/null || echo "Database ${PG_DB} already exists or created"

DB_URL="postgresql+psycopg://${POSTGRES_USER:-postgres}:${POSTGRES_PASSWORD:-postgres}@${POSTGRES_HOST:-postgres}:${POSTGRES_PORT:-5432}/${PG_DB}"
sed -i "s|sqlalchemy.url = .*|sqlalchemy.url = ${DB_URL}|" /app/server/alembic.ini

echo "Running database migrations..."
cd /app/server && alembic upgrade head

echo "Starting mem0 server..."
exec uvicorn main:app --host 0.0.0.0 --port 8000