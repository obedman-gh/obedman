# Security status and reporting

As of 2026-11-07, a PostgreSQL credential previously committed in `api/main.go` is **exposed**. Active code now reads environment configuration. **Production rotation and revocation have not been verified.** Do not reuse the historical credential, including in development.

Local Gitleaks 8.30.1 scanned all reachable Git history with full value redaction and found one historical occurrence. `security/known-history.json` records its exact finding fingerprint only. CI reports that known exposure while failing for every active-source finding and every new historical fingerprint. This exception records unresolved exposure; it is not a declaration that history or credentials are safe. Remove the exception after verified cleanup. No author emails, secret values or raw historical reports are committed.

## Owner rotation procedure

1. Restrict database network access while investigating. Review connection/audit activity from the exposure period with authorized staff. Do not export guestbook contents into logs or tickets.
2. Create separate runtime, moderator and migration login roles using [role setup](ops/roles.sql). Generate unique credentials in a secret manager. Enter passwords using an interactive protected `psql` session (`\password`), without putting values on the command line.
3. Take a protected backup. Apply migrations as the migration owner, then apply [grants](ops/grants.sql). A legacy table may need ownership transferred to the migrator first. Validate production certificate identity and permissions.
4. Configure the new runtime connection in the root-owned environment file, deploy the matched frontend/backend release, and check readiness. Switch the moderator connection separately.
5. Disable login for the old role and terminate its existing sessions after confirming no other application needs it. Credential changes alone do not close already authenticated sessions. Remove obsolete privileges after transferring owned objects safely.
6. Verify that the old credential cannot establish a new connection. Record role identifiers, timestamps, operator and pass/fail results in a private incident record, without secret values. Only then mark rotation verified.

## Owner history cleanup

Rotation takes precedence. History cleanup cannot invalidate copies, forks, caches or old clones.

Freeze repository writes and coordinate with collaborators. Work in a new protected mirror clone, preserve an encrypted recovery copy with restricted access, and use `git-filter-repo` with a private replacement file to remove the exposed value from all affected refs. Never paste the value into commands or reports. Run `gitleaks git --redact=100 --log-opts=--all` on the sanitized mirror without the known-exposure exception. Review the resulting changes and object reachability.

Remote replacement requires a separate coordinated owner action under GitHub branch-protection and support procedures; this task does not force-push, rewrite the current checkout, or remove production data. Ask GitHub to address sensitive cached views/PR refs where applicable, and have collaborators reclone the sanitized repository. Re-scan fresh clones and remove the exact fingerprint exception only once the hosted refs are clean.

## Controls and limits

- Production PostgreSQL URLs require `sslmode=verify-full`. Plaintext TCP, opportunistic TLS and Unix socket production configurations are rejected; no socket exception is implemented.
- Same-origin JSON writes, server-owned fields, length/body limits, parameterized SQL, moderation and bounded reads protect the public data path.
- Requests are limited per process. A distributed attacker can still submit pending spam. Use Cloudflare/WAF limits and a shared limiter before scaling beyond one instance.
- CSP hashes permit known Astro bootstrap scripts. Inline styles remain allowed for the existing spotlight and design; untrusted HTML is never rendered.
- Application logs contain generated request IDs, route labels, status, duration and sanitized failure categories. No raw errors, DSNs, IP addresses, tokens or message bodies are emitted.

For a suspected issue, contact the repository owner privately through their existing contact channel. Do not open a public issue containing credentials or visitor data.
