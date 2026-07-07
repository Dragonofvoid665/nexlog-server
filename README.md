# NexLog v4 — Production-Grade Go Server

High-performance logistics CMS backend built for **10,000 concurrent users** on **2 CPU / 2 GB RAM**.

## Stack

| Layer | Technology |
|---|---|
| Language | Go 1.22 |
| Database | PostgreSQL 16 (connection pool: 50 open / 25 idle) |
| Migrations | Built-in runner (`migrations/*.up.sql`) |
| Cache | In-memory TTL cache (30s), Redis optional |
| Auth | JWT HS256 + bcrypt passwords |
| Rate limit | `golang.org/x/time/rate` token bucket |
| Compression | Gzip (BestSpeed) |
| Security | CSP, CORS, X-Frame, nosniff, Permissions-Policy |
| Profiler | `net/http/pprof` on :6060 (opt-in) |
| Logging | `log/slog` structured JSON |
| Container | Docker multi-stage, non-root `app` user |
| Proxy | Nginx (keepalive 64, gzip, rate limit) |
| CI/CD | GitHub Actions (test → build → push → deploy) |

## Architecture

```
cmd/server/
  main.go               ← entrypoint, wires everything
configs/
  config.go             ← env vars, fails fast if secrets missing
internal/
  db/db.go              ← PostgreSQL pool (Open)
  migrate/migrate.go    ← SQL migration runner
  repository/           ← data access, parameterised queries, context
  service/              ← business logic, seed, language migration
  handlers/             ← HTTP handlers, all use r.Context()
  middleware/           ← auth, CORS, gzip, rate limit, security headers
  cache/cache.go        ← in-memory TTL cache
  logger/logger.go      ← structured JSON logging (log/slog)
migrations/
  0001_init.up.sql      ← schema + indexes
  0001_init.down.sql    ← rollback
public/
  index.html            ← frontend SPA
  admin.html            ← admin panel
.github/workflows/
  ci.yml                ← test → build → docker push → SSH deploy
```

## Quick Start (Docker)

```bash
cp .env.example .env
# Fill in JWT_SECRET and POSTGRES_PASSWORD in .env
# JWT_SECRET: openssl rand -hex 32

docker compose up -d

# Site:  http://localhost:3000
# Admin: http://localhost:3000/admin  (password: password)
# Health: http://localhost:3000/health
```

## Production Install (Arch Linux)

```bash
scp -r nexlog_v4/ root@YOUR_SERVER:/opt/
ssh root@YOUR_SERVER

cd /opt/nexlog_v4
DOMAIN=yourdomain.com sudo bash install.sh
```

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `JWT_SECRET` | ✅ yes | — | Min 32 chars. Server exits if missing |
| `DATABASE_URL` | ✅ yes | — | `postgres://user:pass@host:5432/db?sslmode=disable` |
| `POSTGRES_PASSWORD` | ✅ yes | — | Used in docker-compose |
| `PORT` | no | `3000` | HTTP listen port |
| `APP_ENV` | no | `production` | `development` enables text logging |
| `DB_MAX_OPEN` | no | `50` | Max open DB connections |
| `DB_MAX_IDLE` | no | `25` | Max idle DB connections |
| `RATE_LIMIT_PER_MIN` | no | `120` | API requests per minute per IP |
| `ALLOWED_ORIGINS` | no | `""` | CORS: comma-separated origins |
| `ENABLE_PPROF` | no | `false` | Exposes profiler on :6060 |
| `MIGRATIONS_DIR` | no | `./migrations` | Path to SQL migration files |

## Key Commands

```bash
# Logs
docker compose logs -f

# Restart
docker compose restart

# DB backup
docker exec nexlog-postgres pg_dump -U nexlog nexlog > backup_$(date +%Y%m%d).sql

# DB restore
docker exec -i nexlog-postgres psql -U nexlog nexlog < backup_20250101.sql

# Run profiler (set ENABLE_PPROF=true first)
go tool pprof http://localhost:6060/debug/pprof/heap

# Enable Redis cache (optional)
docker compose --profile cache up -d
```

## Security Checklist

- [ ] Change default admin password at `/admin` → Security
- [ ] Set strong `JWT_SECRET` (≥32 chars, random)  
- [ ] Set strong `POSTGRES_PASSWORD`
- [ ] Configure `ALLOWED_ORIGINS` if needed
- [ ] Install SSL: `DOMAIN=example.com bash install.sh`
- [ ] Review `RATE_LIMIT_PER_MIN` for your traffic pattern
