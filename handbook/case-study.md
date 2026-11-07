# Portfolio guestbook hardening

## Problem

At baseline commit `68e37c5`, a public JSON request was decoded directly into the ORM model. IDs/timestamps were client-writable, inserts could return 201 despite a database failure, all rows were publicly listed without bounds, and schema changes ran during startup. A database credential was in active source and Git history. The README described security/performance without supporting measurements.

## Changes and reasons

- Strict name/message DTO and checked parameterized insert make success depend on persistence. Server SQL owns ID/time/status. Unknown fields are rejected instead of silently ignored so mistakes and field-injection attempts are testable.
- Pending moderation with host CLI avoids building a public admin authentication system for a small portfolio. Audit and mutation share a transaction; old rows require review before publication.
- Keyset pagination plus `(created_at,id)` ordering avoids duplicates at timestamp ties. A partial approved-only index supports the access path. The frontend can load the next page and renders visitor content as text.
- Explicit versioned migrations preserve old data, serialize concurrent runs and check applied SQL hashes. Runtime credentials need no DDL. The first migration preserves historically invalid rows rather than deleting them to satisfy new constraints.
- Standard HTTP/SQL APIs keep lifecycle and failures visible with fewer dependencies. Same-origin deployment/proxy removes broad CORS needs. Bounded requests, database/HTTP deadlines, a finite pool, request IDs, safe logs, CSP and trusted peer rules cover the principal small-app operational risks.
- Frontend handles HTTP/network/JSON failures, loading and retry without clearing user input. A browser test exposed an early-hydration race; disabling the form until React attaches prevents that interaction window.
- A targeted `golang.org/x/text` update addresses [GO-2026-6629](https://pkg.go.dev/vuln/GO-2026-6629) and [GO-2026-5970](https://pkg.go.dev/vuln/GO-2026-5970); the required `x/sync` transitive version also changed. No unrelated frontend upgrades were made.

## Evidence and limits

[Verification](verification.md) records local PostgreSQL integration, 22 desktop/mobile browser cases, static build, dependency/secret scans and a two-row synthetic restore. The code/tests and workflow are reviewable; no traffic, uptime, benchmark or coverage percentage is claimed.

The limiter is intentionally process-local and moderation is manual. A committed insert followed by a lost response can be duplicated by retry. The CSP keeps inline styles for the original visual behavior. Production rollout, old-login revocation, Ubuntu service checks and off-host restore remain owner tasks. These limits are documented instead of calling the application universally production secure.
