# Verification record

Date: **2026-11-07**, Asia/Makassar. Base Git commit: `68e37c5`. Results below describe the current hardening working tree, not an already pushed commit or deployed production release. Existing user edits in `.gitignore`, `dev/src/pages/index.astro`, screenshots and sounds were retained.

Environment: Windows, Node 24.21.0, npm 11.19.0, Go 1.27.3, local PostgreSQL/client 16.14, Python 3.14.8, Playwright 1.65.0 / Chromium 158. A separate loopback PostgreSQL cluster on port 55432 under ignored `.local/` was used, with disposable test databases and synthetic records only. No production URL received automated requests or test messages.

| Check | Observed result and scope |
| --- | --- |
| Clean `npm ci` | Passed with the committed exact-version dependency manifest/lockfile |
| `npm audit --audit-level=low` | 0 known vulnerabilities, including development dependencies |
| `npm run build` | Passed; Astro static output generated |
| Go `gofmt`, `vet`, build, `go mod verify` | Passed |
| `TEST_DATABASE_URL=… go test -count=1 ./...` | Passed, including real PostgreSQL integration; no database skips |
| DTO/input tests | Malformed/multiple JSON, wrong media type/types, empty/whitespace, Unicode boundaries, NUL/invalid UTF-8, body ceiling, extra fields and client ID/time/status injection |
| Failure/visibility tests | Unit and actual missing-table queries/inserts return 503; pending/rejected excluded; empty list returns an empty array; liveness/readiness separated |
| Moderation/migration tests | Existing IDs/text/times preserved, old rows pending, fresh schema, repeat migration, concurrent advisory lock, checksum tampering, atomic migration failure, approval/rejection/deletion and audit rollback |
| Runtime privileges | A temporary restricted NOLOGIN role passed readiness/create/list with only SELECT, name/message INSERT and sequence USAGE; direct status INSERT, UPDATE, DELETE, DDL and audit writes were denied |
| Limiter/proxy/log tests | Fixed-window expiry/capacity, 429/Retry-After, forged X-Forwarded-For and untrusted CF header ineffective, explicit trusted peer behavior, logs exclude input/query/IP/raw driver error |
| Browser regression suite | **22 passed**, 11 cases on desktop Chrome + Pixel 7 viewport; actual Go server serves the build |
| Browser scope | Safe HTML text, page loading/pagination, POST and GET HTTP/network/invalid-JSON failures, retained input, loading/duplicate guards, pending notice, Unicode limits, responsive width, CSP/hydration and image error fallback |
| Full browser/database flow | Both viewports sent synthetic posts through the actual API, verified pending invisibility, approved with the host CLI, saw text, rejected and verified disappearance, then deleted the synthetic rows |
| Astro development proxy | `/api/live`, `/api/ready`, `/api/guestbook` each returned HTTP 200 through `127.0.0.1:4321` to Go |
| Workflow checks | YAML parsed and actionlint 1.7.7 passed (external ShellCheck/pyflakes integrations disabled); not a hosted CI run |
| Synthetic restore | Passed: custom dump/restore into another unique DB, two row representations/counts equal, legacy migration twice, two pending, one approval, readiness and approved-only GET verified |
| Backup helper | `ops/backup.sh` syntax check and execution passed against the synthetic test DB; created a private custom-format dump and checksum; no production backup was accessed |
| Active secret checks | Narrow low-entropy password scan + Gitleaks 8.30.1: 0 active findings |
| Reachable history scan | 1 exposed historical occurrence, exact fingerprint recorded; no new finding outside that exception. Rotation remains unverified |
| `govulncheck` v1.9.0 | Initially found GO-2026-6629/GO-2026-5970 in x/text 0.29.0; after targeted update to 0.41.0: **No vulnerabilities found** |

The first browser run caught a hydration race: automated typing could reach SSR controls before React attached and reset the values. The form now stays disabled until hydration. A repeat full run passed. An initial `npm ci` while the development server was using a native module hit a Windows file lock; stopping that server and repeating the clean install succeeded. These failures were resolved, not counted as passing checks.

## Not verified or not performed

- `go test -race`: unavailable on this Windows environment because CGO is disabled and no C compiler was found. Normal Go tests passed. CI requires the race detector on Ubuntu; that hosted run is still pending.
- GitHub Actions workflow is added with SHA-pinned actions, exact primary runtime versions, a digest-pinned PostgreSQL image and `contents: read`. It has not run on GitHub because these local changes have not been pushed. PostgreSQL client packages and the Ubuntu runner still receive upstream OS updates.
- Ubuntu systemd deployment, production database TLS/least-privilege roles, tunnel configuration, production backups, off-host encryption/retention and real disaster recovery have not been verified. Runbooks/templates are available, not evidence of deployment.
- Historical login rotation/revocation and hosted Git history cleanup require owner access and evidence. No force-push, production deletion or production configuration change was performed.
- There is no traffic, performance benchmark, uptime or coverage percentage claim. Browser geometry/CSP checks do not prove every possible browser, screen size or assistive technology works.

Reproduction commands and required variables are in the [README](../README.md), [CI workflow](../.github/workflows/ci.yml) and [operations guide](operations.md). Ignored `.local/` files hold only local tooling/test state; do not publish that directory or test traces as production artifacts.
