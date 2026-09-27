# Forum

A forum where users publish posts, comment on them and react to them. Written in Go on the standard
library alone, no web framework, `net/http` with pattern-based routing, backed by PostgreSQL.

It began as a coursework project (Software Engineering INT3105 UET-VNU) and has since
been reworked: the database moved to PostgreSQL, full-text search was added, image upload via a self-hosted MinIO AIStor, and the JSON API (with rate limited) was rebuilt into layers while
the server-rendered pages stayed.

---

## 1. The original project

A forum server in Go: cookie-based sessions, posts grouped by category, comments,
like/dislike reactions, and pagination over the post list. Server-rendered HTML only, every page is
built on the server from `text/template`.

### Authors

- Abdelhamid Bouziani
- Hamza Maach
- Omar Ait Benhammou
- Mehdi Moulabbi
- Youssef Basta
<!-- 
### Core features

**Authentication & sessions**

- Cookie-based authentication, passwords stored as bcrypt hashes.
- The session is re-validated on every request (`ValidSession()`), which joins `sessions` against
  `accounts`.

**User interaction**

- Create posts and attach each to one or more categories.
- Comments, and like/dislike reactions on both posts and comments.
- Comment throttling as spam protection.

**Filtering & pagination**

- `/category/{id}` — posts in one category
- `/mycreatedposts` — posts written by the signed-in account
- `/mylikedposts` — posts that account reacted to
- Pagination through a 1-based `PageID` query parameter, 10 rows per page.

### Performance results reported by the original authors

The upstream project load-tested its own optimisation work — the SQLite→MySQL move, an in-memory
cache in front of the post-list pages, and a Prometheus/Grafana stack — at 100 virtual users across
three scenarios: read-only, write-only, and a 50/50 mix.

**These numbers describe that version, not the code in this repository.** The cache and the
monitoring stack are gone from here, and the database is PostgreSQL. They are kept because the full
report is still the clearest write-up of the problem this fork started from.

| Scenario | Metric | Before | After |
| :--- | :--- | :--- | :--- |
| **Overall** | Total requests | 10,628 | 12,655 |
| | Request rate | 49.34/s | 59.15/s |
| | Failed request rate | 22.59% | **0.00%** |
| **Read only** | Response time P95 | 642.91 ms | **8.84 ms** |
| | Response time avg | 307.47 ms | 7.26 ms |
| | Response time max | 1,464 ms | 233.64 ms |
| | Success rate | 99.63% | 100.00% |
| **Write only** | Response time P95 | — | 647.18 ms |
| | Response time avg | — | 161.89 ms |
| | Response time max | — | 929.90 ms |
| | Success rate | 0% (all failed) | **100.00%** |
| **Mixed (50/50)** | Response time P95 | 5,023 ms | **82.74 ms** |
| | Response time avg | 1,434 ms | 40.95 ms |
| | Response time max | 10,658 ms | 658.21 ms |
| | Create-post success rate | 0% | 100.00% |

The upstream project's full report is no longer kept in this repository; the table above reproduces
its headline result.

--- -->

## 2. Key improvements in this fork

### 2.1 PostgreSQL 16 instead of SQLite

SQLite locks the whole database file on write, so one insert blocks every reader.

PostgreSQL locks
rows, which is what makes concurrent posting work. It also runs as a standalone server process
rather than an embedded file-based engine, so it scales far better under concurrent, write-heavy
workloads.

- **Indexing** on what the list and filter queries touch: post timestamps, `user_id`, `post_id`, the
  polymorphic `likes (user_id, target_type, target_id)` key.

### 2.2 Full-text search

Added full-text search. Used PostgreSQL's
  full-text search instead of a naive `LIKE '%keyword%'`: a weighted `tsvector` column
  (title > content) backed by a GIN index keeps queries fast regardless of table size, with
  `ts_rank` surfacing title matches 2.5× above content matches.

### 2.3 Pure SSR to  hybrid SSR + JSON API

Built a versioned JSON API (`/api/v1/…`) alongside the existing SSR pages: SSR still handles
  the initial page for SEO and fast first paint, while interactions (voting, commenting,
  filtering) go through the API and patch the DOM directly — avoiding a full page re-render for
  every action.

### 2.4 Rate limiting

Added rate limiting at three layers: a global token bucket, per-user/per-IP fixed windows, and
per-endpoint limits on sensitive routes (sign-in, sign-up, post creation, upload, search) to
prevent abuse and brute-force attempts.

### 2.5 Image upload: valet key on MinIO

* Added image upload support using the valet key pattern: client-side checks (size, extension,
  MIME, magic bytes) reject bad files early, then the browser uploads directly to MinIO via a
  pre-signed URL.
* Uploaded files land in a private quarantine bucket first and are only re-validated (true size,
  magic bytes, actual image decode) and promoted to the public bucket after a confirmation step.

## 3. Technology stack

| Area | Choice |
| :--- | :--- |
| Language | Go 1.25 |
| HTTP | `net/http` standard library
| Templating | `text/template` |
| Database | PostgreSQL 16 |
| DB driver | `jackc/pgx/v5` via `database/sql`, `jmoiron/sqlx`|
| Search | PostgreSQL native FTS — `tsvector` + GIN + `websearch_to_tsquery` |
| Object storage | MinIO AIStor (S3-compatible) via `minio-go/v7`|
| Auth | Cookie sessions, `golang.org/x/crypto/bcrypt` for password hashes |
| Identifiers | `google/uuid` |
| Rate limiting | `golang.org/x/time/rate` token bucket + fixed windows |
| Logging | `rs/zerolog` |
| Containerisation | Docker + Docker Compose |

---

## 4. Installation & running

### Configuration


| Variable | Purpose |
| :--- | :--- |
| `DB_SOURCE` | Full PostgreSQL DSN (URL or key=value). Wins over the parts below. |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE` | Connection parts |
| `DB_MAX_OPEN_CONNS` etc. | Connection pool sizing |
| `MINIO_ENDPOINT` | Address the **server** dials.|
| `MINIO_PUBLIC_ENDPOINT` | Address the **browser** dials; pre-signed URLs are signed against it |
| `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_USE_SSL`, `MINIO_REGION` | Credentials and signing region |
| `MINIO_QUARANTINE_BUCKET`, `MINIO_MEDIA_BUCKET` | Private staging bucket and public media bucket |
| `GLOBAL_RPS`, `GLOBAL_BURST`, `USER_RPM`, `IP_RPM` | Rate limiter windows |
| `LOGIN_ATTEMPTS`, `REGISTER_ATTEMPTS`, `POSTS_PER_HOUR`, `COMMENTS_PER_HOUR`, `UPLOAD_REQUESTS_PER_MINUTE`, `SEARCH_REQUESTS_PER_MINUTE` | Per-endpoint buckets |
| `ENVIRONMENT`, `LOG_LEVEL` | `development` switches zerolog to console-pretty; `debug` raises the level |
| `BASE_PATH` | Empty locally, `/app/` in Docker. Every template and asset path is built from it. |


### With Docker Compose

<!-- Compose starts three services: `app` (published on **`:8080`**), `db` (PostgreSQL 16, published on
host **`:5433`** so it does not collide with a local Postgres) and `minio` (S3 API on `:9000`, web
console on `:9001`).

**1. Put the MinIO license in the repository root.** The object-storage service is **MinIO AIStor**,
not the community image, and it refuses to start without a license file at `./minio.license`. The
file is a credential: it is listed in both `.gitignore` and `.dockerignore`, so it is neither
committed nor baked into the image — compose bind-mounts it read-only at run time. -->

```bash
cp .env.example .env    # optional: the compose defaults work as-is
docker compose up --build -d
```

The app is then on <http://localhost:8080> and the MinIO console on <http://localhost:9001>.

```bash
docker compose logs -f app     # service name is "app"
docker compose down
```

<!-- Note on startup: `app` waits for `db` to be healthy but only for `minio` to have *started*, because
AIStor validates its license before it listens. The resulting race is absorbed by
`EnsureBucketsWithRetry` (30 attempts, one second apart) in `cmd/main.go`, which then fails fatally if
storage is still unusable. -->

### Without Docker

```bash
go run ./cmd/main.go              # server on :8080
go run ./cmd/main.go --migrate    # drop + recreate the schema, then exit
go run ./cmd/main.go --seed       # schema + demo data, then exit
```

Flags are honoured only when `BASE_PATH` is unset, and only one at a time. A local run talks to the
database described by `DB_SOURCE`, falling back to the `DB_HOST`/`DB_PORT`/… parts — for the
compose-provided Postgres that means `DB_PORT=5433`. Uploads stay disabled until `MINIO_ENDPOINT` is
set; everything else works without it.

Use `--migrate` to reset the database. `--drop` is not an option on PostgreSQL: it still tries to
delete a SQLite file left over from an earlier design. The schema file drops every table itself, so
`--migrate` is the reset.

### Demo accounts

Seeded by `--seed` (and automatically on every Docker start):

| Username | Password |
| :--- | :--- |
| `alice` | `password1` |
| `bob` | `password2` |

---
<!-- 
## 6. Known limitations

Stated up front rather than discovered later:

- **No tests.** There are no `*_test.go` files in the repository.
- **Docker wipes the database on every start.** The image's `CMD` is
  `./forum --migrate && ./forum --seed && ./forum`, and `main.go` seeds again whenever `BASE_PATH` is
  set. Both paths run a schema file that begins with `DROP TABLE IF EXISTS`, so the `db_data` volume
  does not preserve anything. Uploaded objects live in the separate `aistor_data` volume, so they *do*
  survive — and their `medias` rows do not, which leaves orphaned objects behind in `forum-media`.
- **No cache tier.** Every request, including the session lookup the rate-limit middleware performs
  and the navbar's category fetch on every HTML render, is a database round trip.
- **No metrics endpoint.** `MetricsMiddleware` is a pass-through stub; observability is zerolog only.
- **Search is English-only**, with no `unaccent`, so Vietnamese diacritics are not folded.
- **Documentation is this file plus `CLAUDE.md`.** The design documents the project accumulated —
  the architecture target, the refactor plans, the API reference and the original report — have been
  removed rather than left to rot, since several of them described a stack (MySQL, Azure, a cache
  tier, Prometheus) that this tree no longer contains. -->
