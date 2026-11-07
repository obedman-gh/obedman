# Operations

These are owner-run procedures. Local synthetic migration/restore and browser flows have been tested; Ubuntu service installation, the production tunnel, database certificate/roles and off-host backups have not been executed on the real host. Never use the live guestbook for automated test messages.

## Configuration and roles

The server requires explicit `APP_ENV=development|production` and a PostgreSQL `DATABASE_URL` containing a user, host, database and `sslmode`. `LISTEN_ADDR` defaults to `127.0.0.1:3030`; `STATIC_DIR` is relative to the working directory unless absolute. Only `sslmode`, `sslrootcert` and `connect_timeout` URL options are accepted. Connection PG environment overrides are rejected; an optional protected `PGPASSFILE` may provide the password. Do not print environment files or run services with shell tracing.

Production requires `sslmode=verify-full`, a matching certificate hostname, and a trusted CA (supply `sslrootcert` if it is not in system trust). This also applies to loopback TCP. No Unix socket exception is implemented. Arrange PostgreSQL TLS and `hostssl`/SCRAM authentication before setting production mode; do not weaken verification to make startup pass.

On a dedicated database, run [roles.sql](../ops/roles.sql) as owner, assign strong passwords with `\password`, and grant CONNECT only to the intended login roles. Migration role owns the schema objects; runtime has SELECT plus column-restricted INSERT and sequence USAGE only. Moderator has SELECT/UPDATE/DELETE on guestbooks and audit INSERT/sequence USAGE. Apply [grants.sql](../ops/grants.sql) after migrations. The migration login is not in the runtime service environment. Do not grant runtime membership in the other roles or schema CREATE.

Existing GORM installations often have the old login as table owner. Before migration, transfer `guestbooks` ownership to `guestbook_migrator` using the database owner; PostgreSQL transfers the owned sequence along with the table. Verify sequence name with `pg_get_serial_sequence` if it differs from the standard `guestbooks_id_seq`, and adapt the explicit sequence grants. Keep public-schema CREATE revoked. Validate privileges using `has_table_privilege`, `has_column_privilege` and `has_sequence_privilege`, plus a synthetic database test. Avoid testing insert/delete permissions on production data.

## Migrations and compatibility

1. Take a backup and verify the dump can be listed. Schedule a maintenance window: adding a default/check/index to an existing table takes locks.
2. Stop the old service from accepting writes. The old binary exposes pending rows and must not run beside the new moderation schema/API.
3. Use the migration role and run `obedman-api migrate`. All versions run inside one transaction behind a PostgreSQL advisory transaction lock; timeout is 60 seconds. A failure rolls back the entire batch. Existing version checksums must match.
4. Apply runtime/moderator grants. Switch back to runtime credentials, start the paired release, and verify `/api/ready`.

Version 001 creates a fresh table or upgrades the existing `guestbooks` table in place. All old IDs, text and timestamps remain, including historical out-of-limit rows. Every old row becomes pending. New writes have database input constraints; `NOT VALID` avoids deleting legacy invalid rows. Updating a legacy invalid row (including approval) will fail until a restricted operator corrects it. Review content in a private database client, never in application/CI logs. A legacy NULL timestamp is assigned server time only when that row is approved. Version 002 adds metadata-only moderation audit. Re-running migrations is safe; concurrent migrators serialize. A binary refuses startup on a different schema version.

Do not edit an applied migration. Add a new version and update `schemaVersion`. There are no destructive automatic down migrations. Rollback normally means stopping the service and switching the `current` symlink to a previous **moderation-capable**, schema-compatible paired release, then restarting. The original pre-hardening binary is incompatible with the moderation policy. For a future incompatible migration, use a separate approved maintenance plan and restore into another database before cutover; do not overwrite or drop production automatically.

## Moderation, deletion and retention

Use an SSH session limited to the operator account. Load moderator credentials securely and set a nonsecret stable `MODERATOR_ID` (1–80 characters). Keep shell tracing/history away from secret setup.

```sh
./obedman-api moderate list          # First 100 pending IDs/status only, no message contents.
./obedman-api moderate approve 123
./obedman-api moderate reject 123
./obedman-api moderate delete 123
```

IDs above are placeholders. Review name/message through a restricted private DB client by numeric ID; do not paste visitor content into terminal logs, tickets or screenshots. Work through pending batches to reach the rest of the queue, or use a private query by ID range. No anonymous admin HTTP routes exist. Each successful action atomically writes ID/action/operator/time to `moderation_audit`. An absent ID fails and leaves no audit row.

Default retention policy: review pending/rejected rows within 90 days and delete expired rows using the CLI after selecting only IDs/timestamps. Keep approved rows while relevant; review annually. Respond to verified removal requests by identifying the row privately and using `moderate delete`. Do not collect additional identity documents for a public guestbook unless necessary to resolve ambiguity. Keep audit metadata for 365 days, then an authorized owner may purge older metadata in a controlled maintenance transaction. No automated purge is installed. Backups age out separately under the policy below; deletion from the live table does not instantly erase protected backups. Audit IDs do not include message contents.

## Ubuntu release and tunnel

Build using pinned Node/Go versions on a trusted builder:

```sh
cd dev
npm ci
npm run build
cd ../api
go test ./...
go build -trimpath -o ../.local/obedman-api .
```

Package the binary and `dev/dist` into a new immutable `/opt/obedman/releases/<release-id>/` directory. Set a `current` symlink only after validation. Keep the tree readable but unwritable by the service account. Never copy local `.env`, `.git`, backups or source secrets into the release. Create a dedicated unprivileged `obedman` system user; put runtime environment in `/etc/obedman/runtime.env` owned by root with mode 0600. Set absolute `STATIC_DIR=/opt/obedman/current/dev/dist` and `APP_ENV=production`. Install [obedman.service](../ops/obedman.service), run the migration/grants procedure, then `systemctl daemon-reload` and `systemctl enable --now obedman`.

The service has a 10-second application drain and 15-second supervisor stop timeout; database pool closes on exit. Check `systemctl status`, `/api/live`, `/api/ready` and restricted journal logs. Observe readiness and restart counts with host monitoring; no external alerting integration has been configured here.

Configure the existing named Cloudflare Tunnel to proxy to `http://127.0.0.1:3030`. Restrict the origin listener and PostgreSQL firewall. Set `TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128` only when cloudflared is the direct peer on that host. The server then accepts Cloudflare's overwritten `CF-Connecting-IP` from those peers; it never uses arbitrary `X-Forwarded-For`. If the tunnel is in a container or another host, explicitly document and restrict that source before changing trust. Apply edge HTTPS redirection; HSTS is emitted in production. Browser-to-edge and edge-to-origin protections do not secure the database hop.

The limiter is per-process/in-memory (POST 5/minute, GET 60/minute, fixed windows). Restart resets it; multiple replicas multiply allowance. Add edge request controls and a shared limiter before scaling. CSP hashes come from build HTML at startup, so never modify `dist` beneath a running binary. Inline style allowance supports the existing spotlight; the browser regression suite confirms script CSP enforcement and image fallbacks.

## Backup and restore

[backup.sh](../ops/backup.sh) creates a custom-format dump, validates its table of contents, atomically renames it and writes a SHA-256 sidecar with umask 077. Use a restricted backup login with read/sequence rights, root-owned `PGPASSFILE` (0600), TLS verification and a private `BACKUP_DIR` outside the web root. Run nightly under a dedicated service/timer and monitor success/freshness. Dumps include visitor data: never commit, print or upload them to CI artifacts.

After each successful backup, copy it and its checksum to a separate host/account using encrypted backup storage (for example restic over an authenticated remote repository). Store repository credentials and encryption keys in a secret manager; keep a recoverable key copy outside the application host. The application login must not be able to read or delete remote backups. Adopt 7 daily, 4 weekly and 6 monthly recovery points; keep plaintext local staging at most 48 hours after confirmed transfer. Configure and test remote lifecycle/pruning before enabling automatic cleanup. Neither remote storage nor a production backup job has been provisioned by this change.

Restore into a **new isolated database**, never over the live one:

1. Retrieve a protected backup and checksum, verify its SHA-256 and `pg_restore --list`. Use a compatible PostgreSQL client (same/newer major than the dump writer).
2. Create a separate restore database and restrict network access/roles. Run `pg_restore --exit-on-error --no-owner --no-acl --dbname=<isolated-db> <backup-file>`. Review production data only in that authorized private environment; disable publishing/outbound integrations.
3. Compare source checkpoint row counts and schema versions, run migrations with an appropriate role, reapply grants, and exercise readiness and approved-only reads. Review rejected/pending counts, audit counts, sequence continuity and timestamps. Rehearse synthetic insert/moderate/delete in a **synthetic** clone, not by adding test posts to production-derived data.
4. Record evidence without visitor contents. Any production cutover needs separate owner authorization and a rollback window. Keep the original database intact until the owner validates the result.

For a reproducible no-private-data drill, set `PGHOST=127.0.0.1`, `PGPORT`, `PGUSER`, optional `PGPASSFILE`, and `API_BINARY`, then run `python scripts/restore-drill.py` from the repo root. It creates unique source/restore databases ending in `_test`, writes two synthetic legacy rows, dumps/restores, compares full row representations and counts, runs migrations twice, approves one row and verifies readiness/public GET. Finally it removes only databases it successfully created. Actual local result is in [verification](verification.md); this does not prove production backups or off-host recovery.
