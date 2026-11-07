# Architecture and decisions

```text
Browser --HTTPS--> Cloudflare edge --Tunnel--> cloudflared on Ubuntu
                                                  |
                                     loopback HTTP 127.0.0.1:3030
                                                  |
                         Go net/http: Astro static build + /api
                                                  |
                              PostgreSQL TCP with verified TLS
```

This is the intended deployment topology, not a claim that the current live host has been inspected. The edge protects the browser connection and can enforce coarse request rules. The tunnel restricts origin exposure; it does not validate guestbook content, moderate posts or verify the Go-to-database connection. Firewall PostgreSQL to the intended host and bind Go to loopback. Keep the database inaccessible to the public internet.

## Data path

A visitor's form produces JSON with name/message. Go enforces media type, a 16 KiB body ceiling, strict DTO decoding and Unicode limits, then performs one INSERT under a three-second context deadline. PostgreSQL assigns the sequence ID and timestamp; SQL fixes moderation status to pending. Only a successful RETURNING produces 201. A timeout/disconnect after a committed insert can leave the client unsure; retries can create duplicates. This small app does not implement idempotency keys.

An authenticated host operator reviews pending rows through restricted database access and runs the CLI to approve, reject or delete a numeric ID. The mutation and metadata audit insert share a transaction. Message text is not copied into audit logs. GET filters to approved rows and uses descending `(created_at,id)` keyset pagination, backed by a partial index. A cursor is an opaque position, not an authorization token; modifying it cannot expose pending/rejected rows. Concurrent moderation can change which rows are visible between pages; this is not a snapshot export.

## Threats and choices

| Threat | Control | Boundary/limitation |
| --- | --- | --- |
| Exposed database login | Env-only source, safe startup errors, incident/rotation runbook | Old login revocation requires owner evidence |
| Mass assignment | Dedicated DTO, reject unknown keys | API clients must follow the new contract |
| Stored XSS | React text interpolation, JSON encoding, CSP script hashes | Inline styles retained for current design |
| Spam/resource exhaustion | 5 POST/minute/IP, 60 GET/minute/IP; bounded body/pages/pool/queries | In-memory fixed window, burst at a window boundary, reset on restart |
| Spoofed client identity | Peer IP by default; ignore X-Forwarded-For | CF-Connecting-IP used only for explicit trusted direct peers |
| Hidden persistence failure | Checked query/scan/insert/commit/migration errors, 503 and request IDs | Errors are categorized without driver detail |
| Accidental schema damage | Explicit versioned transaction, advisory lock, checksums | DDL can lock a large legacy table; schedule maintenance |
| Process failure | Separate live/ready, finite HTTP timeouts, shutdown and connection close | Service supervision and monitoring still need host configuration |

The limiter stores at most 10,000 route/client buckets and fails closed while full; expired buckets are reclaimed. Its five-request POST allowance includes rejected requests. It uses no visitor IP persistence. Each instance has its own allowance; restart clears it. Proxy trust must list only `cloudflared`'s actual source network, usually the two loopback host prefixes. A local process with access to that listener can spoof CF headers; host access is part of the trust boundary. Never trust arbitrary networks to recover client IPs.

No broad CORS policy is enabled: browser and API share an origin; Astro proxies API calls during development. Cross-origin JSON writes with a mismatched Origin are rejected. This is a public unauthenticated write API, so Origin checks do not replace spam controls.

## Structure and trade-offs

`api/config.go`, `http.go`, `store.go`, `migrate.go` and `moderate.go` separate concrete concerns without a framework/repository hierarchy. Standard `net/http` replaces Fiber, and parameterized pgx-backed `database/sql` replaces GORM; this removes ORM startup DDL and makes HTTP lifecycle and database errors explicit. Astro, React, Go, PostgreSQL and portfolio styling remain.

One-row create/list operations need no extra transaction. Migration batches and moderation/audit changes do. Pool limits are 10 open / 5 idle, lifetime 30 minutes, idle lifetime 5 minutes. HTTP header/body read limits are 5/10 seconds, writes 15 seconds, keepalive idle 60 seconds; shutdown drains for 10 seconds. No database writes run during startup except explicit `migrate`.

Build assets must stay immutable while a process runs: script hashes are computed from static HTML on startup. Release directories pair one binary and one frontend build; restart the process for each release. Browser tests verify real CSP enforcement and hydration against that build.
