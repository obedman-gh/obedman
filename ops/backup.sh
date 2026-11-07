#!/usr/bin/env bash
set -euo pipefail
umask 077
# Configure PGHOST/PGPORT/PGUSER/PGDATABASE/PGPASSFILE via a root-owned service.
# For remote PostgreSQL: PGSSLMODE=verify-full and PGSSLROOTCERT are required.
: "${BACKUP_DIR:?set a private absolute backup directory}"
: "${PGDATABASE:?set dedicated database}"
test "${BACKUP_DIR:0:1}" = /
mkdir -p -- "$BACKUP_DIR"
stamp=$(date -u +%Y%m%dT%H%M%SZ)
file="$BACKUP_DIR/guestbook-$stamp.dump"
partial=$(mktemp "$BACKUP_DIR/.partial.XXXXXX")
trap 'rm -f -- "$partial"' EXIT
pg_dump --format=custom --no-owner --no-acl --file="$partial"
pg_restore --list "$partial" > /dev/null
mv -- "$partial" "$file"
sha256sum "$file" > "$file.sha256"
# Off-host encrypted transfer and retention are deliberate operator tasks; see handbook/operations.md.
