# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build & Run
```bash
# Full stack (recommended)
docker compose up --build -d

# View logs
docker logs -f forum-con

# Stop
docker compose down
```

### Local Development (no Docker)
```bash
# Migrate schema + seed demo data
go run . --migrate
go run . --seed

# Run server (flags mode)
go run .
```

### Database Commands
```bash
# Only schema changes (prod/Docker)
go run . --migrate

# Add demo data (dev only)
go run . --seed

# Drop schema (dev only)
go run . --drop
```

### Metrics & Monitoring
```bash
# Prometheus metrics endpoint
curl http://localhost:9090/metrics

# Grafana (default admin:admin)
open http://localhost:3000
```

### Single Test (if any exist)
```bash
go test ./server/... -run TestName -v
```

### Quarantine Watcher
```bash
# Enable in docker-compose with ENABLE_QUARANTINE_WATCHER=true
```

## Architecture Overview

### High-Level Flow

1. **Entry Point** (`cmd/main.go`)
   - Loads `.env` (or uses env vars)
   - Connects to MySQL (with retry + pool config)
   - Initializes two caches: session (5s TTL) and category (5m TTL + auto-refresh from DB)
   - Sets up Azure Blob Storage (if `AZURE_STORAGE_CONNECTION_STRING` present)
   - Starts Quarantine Watcher (background goroutine)
   - Configures rate limiting middleware
   - Starts metrics collection goroutines
   - Starts HTTP server on `:8080` (metrics on `:9090`)

2. **Database Layer**
   - MySQL 8 (InnoDB)
   - Heavy use of `post_materialized_view` (pre-computed like/dislike/comment counts + categories_str)
   - All queries wrapped with `QueryWithMetrics` / `ExecWithMetrics` for Prometheus
   - Transactions for writes (posts, comments, reactions)

3. **Caching Layer**
   - LRU cache (`github.com/hashicorp/golang-lru/v2`)
   - Dedicated `CategoryCache` with `LoadCategories` + auto-refresh
   - Session cache for quick user lookup

4. **Storage Layer**
   - Azure Blob Storage (with SAS tokens via valet key pattern)
   - Quarantine container → Production container workflow
   - Quarantine Watcher (background worker) validates files before promotion

5. **Rate Limiting**
   - Global (Token Bucket)
   - Per-user (Fixed Window)
   - Per-IP (Fixed Window)
   - Endpoint-specific (login/register/post/comment/upload)
   - Applied at middleware level

6. **Request Flow** (typical)
   - Client → Rate limiting middleware → Controller → Model → Database (or cache) → Response
   - Image uploads: Client validates → Request SAS token → Direct upload to Azure → Quarantine watcher promotes

### Key Patterns Used

- **Materialized View** for read performance (avoids N+1 queries)
- **Retry pattern** on database operations (configurable)
- **Validation Pipeline** (client + server side for uploads)
- **Metrics** everywhere (Prometheus counters/histograms)
- **Docker + docker-compose** for full stack

The architecture is designed for high throughput with strong security (quarantine + SAS + rate limiting) and observability (Prometheus + Grafana). 

To understand any feature deeply, you will usually need to read at least:
- The relevant model function
- The controller handler
- The route definition
- The middleware that wraps it
- The config file for limits/cache

This CLAUDE.md was generated from the complete codebase. Future Claude instances can reference it directly.