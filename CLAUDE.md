# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Full stack (Docker)
```bash
docker compose up --build -d          # app + MySQL + Prometheus + Grafana
docker logs -f forum-con              # app logs (container name from commands.sh)
docker compose down
```
The Docker image's `CMD` runs `./forum --migrate && ./forum --seed && ./forum`, so the container
always rebuilds the schema and reloads demo data on start. `docker-compose.yml` also sets
`BASE_PATH=/app/`, which is how `cmd/main.go` detects Docker mode.

### Local development (no Docker)
```bash
go run ./cmd/main.go                  # server on :8080, metrics on :9090
go run ./cmd/main.go --migrate        # create schema only
go run ./cmd/main.go --seed           # create schema + demo data
go run ./cmd/main.go --drop           # dev only; deletes server/database/database.db
```
Flags are only honored when `BASE_PATH` is unset (i.e. not Docker). Passing exactly one flag of
`--migrate|--seed|--drop` runs setup and exits; no flag starts the server.

### Build / verify
```bash
go build ./...            # compiles clean — all layers wired (auth/posts/comments JSON API are live)
go vet ./...
go test ./...             # no test files exist yet
go test ./server/usecase/... -run TestName -v   # pattern for adding the first unit tests
```

### Observability
```bash
curl http://localhost:9090/metric        # Prometheus endpoint (note: path is "metric", not "metrics")
open http://localhost:3000               # Grafana, admin:admin
```
Prometheus/Grafana run as containers in `docker-compose.yml`; the app's metrics mux is registered
inside `cmd/main.go`.

## Current state: layered API + SSR hybrid (read this first)

The migration from SSR monolith to layered architecture is **complete**. The JSON API v1 routes
(`/api/v1/auth/*`, `/api/v1/posts/*`, `/api/v1/posts/{id}/comments`, `/api/v1/comments/*/reactions`)
are now **live and wired** in `server/routes/routes.go`. The SSR page routes (`/`, `/post/{id}`,
`/category/{id}`, `/mycreatedposts`, `/mylikedposts`, login/register pages) remain for HTML
rendering and SEO first-paint.

The recommended authority for understanding current request flow and wired endpoints is
`guide.md` (repo root), which maps the actual running code. `docs/api-ssr.md` marks CURRENT
routes that are in the codebase vs PROPOSED routes that are planned.

### What is actually wired

| Layer | File | Status |
|---|---|---|
| Auth API | `server/controller/auth_controller.go`, `auth_routes.go` → `usecase.AuthUsecase` | LIVE |
| Post API | `server/controller/post_api_controller.go`, `post_routes.go` → `usecase.PostUsecase` | LIVE |
| Comment API | `server/controller/comment_controller.go` → `usecase.CommentUsecase` | LIVE |
| SSR pages | `server/controller/post_controller.go` (free-function handlers calling repos directly) | LIVE |

### Schema status

The sole active scheme is in `server/repository/postgresql/migration/20260916094300_schema.sql`:
- `accounts` (CHAR(36) UUID primary key) replaces `users`
- `likes` table with `target_type ENUM('post','comment')` replaces `post_reactions`/`comment_reactions`
- `medias` table for images (FK from `posts.media_id`)
- `profiles` table for user profiles
- Denormalized counts on `posts` (`like_count`, `dislike_count`, `comment_count`) — no materialized view

`server/database/sql/schema.sql` no longer exists. `config.CreateTables` reads the migration file.

## Architecture

### Request flow

Two parallel branches share one mux (`server/routes/routes.go`):

```
main → mysql connect → session cache (5s TTL) + category cache (5m TTL, auto-refresh)
     → Azure storage init (only if AZURE_STORAGE_CONNECTION_STRING set)
     → quarantine watcher goroutine (if ENABLE_QUARANTINE_WATCHER=true)
     → MetricsMiddleware → RateLimitMiddleware → routes.Routes

SSR branch (HTML):     mux → controller free-fn → repository/mysql directly
                            → utils.RenderTemplate → web/templates/*.html

JSON API branch:       mux → controller → usecase → repository/mysql
                            → writeJSON / writeAppError (envelope)
```

The SSR controllers deliberately bypass the usecase layer and call repositories directly (to keep
page rendering simple and SEO-stable); the JSON API controllers go through the full clean chain.
Both are intentional — see `guide.md` §11. Usecases receive `AuthUsecase` by constructor injection
(`NewPostUsecase(db, authUc)`) to validate sessions without a service locator.

### Layer rules for new code (from `docs/ref/arch.md`)

```
handler/controller → usecase → repository interface → repository/mysql → database/sql
                             → cache / storage adapters
```
- Controllers bind and validate, then call a usecase. They must not run SQL or hold business logic.
- Usecases own business rules, authorization and transaction boundaries. They return DTOs plus
  errors, and must never touch `http.ResponseWriter`.
- Repositories own all SQL, `rows.Scan` and row→domain mapping. Keep `database/sql` with
  parameterized queries and the metrics wrappers — the project deliberately does **not** use an ORM.
- Use `database.QueryWithMetrics` / `ExecWithMetrics` / `ExecWithMetricsTx` (and the
  `...WithMetricsAndError` variants) rather than `db.Query` directly: these emit the Prometheus
  labels the dashboards depend on.

### Error handling contract (new layers)

`usecase.AppError` carries `Status`, `Code`, `Message`, `Details`. Define sentinel errors with
`NewAppError` (e.g. `ErrNoSession`, `ErrEmailAlreadyTaken`), return them from usecases, and let
`usecase.ToHTTP(err)` map them to status/code/message for the controller. Sharing error constants
happens in `server/usecase/errors.go` (`CodeValidationError`, `CodeUnauthorized`, ...). Only
`*AppError` gets its own status — everything else becomes a generic 500, so unexpected errors never
leak SQL detail.

Note the package-name/import-alias mismatch: `server/repository/postgresql/user/` declares
`package auth` and is imported as `userRepository`. Don't "fix" it blindly; grep callers first.

### Delivery: hybrid SSR + JSON (current)

This is an intentional **monolith hybrid**, not a split frontend deployment — there is no separate
frontend app, and `guide.md` argues against creating one.
- SSR page routes (`/`, `/post/{id}`, `/category/{id}`, `/mycreatedposts`, `/mylikedposts`, form
  pages) stay HTML for SEO and first paint, rendered via `utils.RenderTemplate` + `web/templates/`.
- Interactions (reaction, comment, delete, upload) from `web/assets/js/*` are JSON/AJAX and do not
  reload the page.
- Legacy interaction paths (`/post/postreaction`, `/post/delete/{id}`) return JSON but are
  transitional; the new `/api/v1/*` endpoints use the clean envelope format.
- `/api/v1/auth/*` endpoints provide JSON equivalents for `/signin`, `/signup`, `/logout`.
- `/api/v1/posts`, `/api/v1/comments`, `/api/v1/me/*` endpoints replace legacy SSR handles.

When adding an API-v1 endpoint, register it alongside the legacy route rather than replacing it, and
follow the documented envelope: `data` / `pagination` on success,
`{"error": {"code", "message", "details"}}` on failure, HTTP statuses mapped 400/401/403/404/409/429/500.

### Data model and performance design

The system's performance story (see README benchmark table) rests on **denormalized counts** and
cache-aside pattern:

- **Denormalized counts on `posts` table** — `like_count`, `dislike_count`, `comment_count` are
  updated atomically in the same transaction as the `likes` table changes. This eliminates the need
  for COUNT(*) queries across reaction tables on every post list.
- **Cache-aside** over an LRU (`cache.AppCache`, size/TTL in `server/config/cache_config.go`) with
  keys like `post_{id}`; invalidate *after* commit.
- **Retry** (`server/utils/retry/`) applied only to transient DB/network errors — never to
  validation, authorization, or non-idempotent writes of unknown commit status.

Domain types live in `server/model/` as pure structs with typed IDs (`PostID`, `AccountID`,
`CategoryID` — `AccountID` is a `uuid.UUID` alias). They must not import `net/http` or SQL.

### Rate limiting

Four tiers, all configured by env vars in `server/config/ratelimit_config.go` and applied as
middleware in `routes.go`: global token bucket, per-user fixed window, per-IP fixed window, and
endpoint-specific buckets (login/register/post/comment/upload). Defaults live in the config file;
`.env.example` lists the overridable names.

### Upload pipeline (Azure)

Client validates (size, extension, MIME, magic bytes in `web/assets/js/validation/`) → requests a SAS
token from `/api/upload/request-url` (gatekeeper in `server/middleware/upload_gatekeeper.go`) →
uploads directly to the **quarantine** container → either the Event Grid webhook
(`WebhookController.HandleBlobCreated`) or the in-process quarantine watcher
(`server/workers/quarantine_watcher.go`, enabled by `ENABLE_QUARANTINE_WATCHER=true`) re-reads the
first bytes, validates the signature, and promotes the blob to the production container. The app
degrades gracefully with uploads disabled when `AZURE_STORAGE_CONNECTION_STRING` is unset.

## Conventions in this codebase

- Paths are built from `config.BasePath` (empty locally, `/app/` in Docker) — use it for any file
  read or template parse, never a bare relative path.
- Package `server/controller/` is named `controllers`. Templates: `RenderTemplate` soft-fails the
  navbar category lookup, and renders `error.html` for page-route failures while interaction routes
  return JSON. Matching that split matters; a page route returning a JSON error is a regression.
- Pagination is `PageID` (1-based, page size 10) on legacy SSR routes. Reads past the end return 404,
  an unparseable `PageID` returns 400 — replicate this rather than "fixing" it, unless the change is
  deliberate and reviewed.
- All write paths invalidate affected cache keys after commit, including list caches per category.
