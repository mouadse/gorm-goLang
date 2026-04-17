# Fitness Tracker API

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/mouadse/gorm-goLang)

Comprehensive Go backend for a complete fitness tracking ecosystem: users, exercises, workouts, nutrition, analytics, and automation.

## Features & Scope

The backend provides a robust API for full-spectrum fitness tracking:

- **User Management**: Profiles, goals, TDEE calculation, and two-factor authentication (2FA).
- **Exercise Library**: Extensive muscle-group focused exercise database with instructions and video URLs.
- **RAG Knowledge API**: Book-ingestion and retrieval endpoints exposed through the Go API for documentation-style Q&A.
- **Workout Tracking**: Set-by-set logging, cardio entries, and workout notes.
- **Workout Intelligence**: Reusable workout templates and long-term training programs.
- **Nutrition Ecosystem**: 
  - Comprehensive food catalog with USDA integration support.
  - Detailed meal logging with food-item joins and micronutrient tracking (19+ vitamins/minerals).
  - Recipe management and favorite foods.
  - Dynamic nutrition targets based on user goals.
- **Analytics & Reporting**: Workout volume analysis, progression tracking, and data export (JSON/CSV).
- **Automation**: Intelligent notifications for low protein, workout reminders, and recovery warnings.
- **Security**: JWT-based session management, bcrypt password hashing, and 2FA secrets.

## Stack

- **Language**: Go 1.25+
- **Database**: PostgreSQL (GORM ORM)
- **Monitoring**: Prometheus & Grafana
- **Infrastructure**: Docker & Docker Compose
- **Documentation**: OpenAPI 3.0 (Swagger UI)

## Project Structure

```text
.
├── api/                # HTTP handlers and server orchestration
├── database/           # Connection logic and schema migrations
├── models/             # GORM entities (User, Workout, Meal, etc.)
├── services/           # Business logic (Auth, Analytics, Notifications)
├── metrics/            # Prometheus metrics middleware
├── monitoring/         # Grafana/Prometheus configurations
├── rag_setup/          # Qdrant-backed RAG API, ingest job, and UI
├── scripts/            # Utility scripts (USDA data import)
└── seed/               # Idempotent development data seeder
```

## Quick Start

### 1. Prepare Environment
From the `Backend/` directory:
```bash
cp .env.example .env
```

Run the orchestration Makefile from `Backend/` for the standalone backend stack. When this repo sits beside the monorepo `Front-End/` checkout, the same commands also include the Nginx-served frontend automatically.

Set at least:
- `JWT_SECRET`
- `OPENROUTER_API_KEY`
- `HF_TOKEN` if your RAG setup needs authenticated Hugging Face downloads

### 2. Start the Full Dev Stack
From the `Backend/` directory:
```bash
make run
```

This now starts, in one command:
- PostgreSQL
- exercise library service
- migrations and seed job
- API and worker
- Prometheus and Grafana
- AI coach UI
- RAG Qdrant, an automatic RAG bootstrap ingest job, the RAG API, and the RAG UI

If `../Front-End` is present, `make run` also starts the production-built React frontend behind Nginx.

### 3. Ingest RAG Documents When Needed
```bash
make rag-ingest
```

`make run` now bootstraps the initial RAG collection automatically from `rag_setup/books/`.
Add or update PDFs after the stack is already running, then rerun `make rag-ingest`.

Query the RAG system through the Go API once the stack is up:
```bash
curl -X POST http://localhost:8082/v1/rag/query \
  -H "Content-Type: application/json" \
  -d '{"query":"What is Kamal and what does it do?","include_sources":true}'
```

If a host port is already occupied on your machine, override it in `.env` instead of editing compose files. The stack now supports `FRONTEND_HOST_PORT`, `API_HOST_PORT`, `EXERCISE_LIB_HOST_PORT`, `COACH_UI_HOST_PORT`, `GRAFANA_HOST_PORT`, `PROMETHEUS_HOST_PORT`, `PGADMIN_HOST_PORT`, `POSTGRES_HOST_PORT`, `WORKER_METRICS_HOST_PORT`, `RAG_API_PORT`, `RAG_UI_PORT`, and `RAG_QDRANT_PORT`.

If RAG is saturating your machine, cap it with `.env` knobs: `RAG_API_CPUS`, `RAG_INGEST_CPUS`, `RAG_QDRANT_CPUS`, `RAG_UI_CPUS`, `RAG_CPU_THREADS`, `RAG_EMBED_THREADS`, `RAG_EMBED_BATCH_SIZE`, `RAG_EMBED_PARALLEL`, `QDRANT_MAX_INDEXING_THREADS`, `QDRANT_MAX_SEARCH_THREADS`, `QDRANT_MAX_OPTIMIZATION_THREADS`, and `QDRANT_OPTIMIZER_CPU_BUDGET`.

## Monitoring & Management

- **Swagger UI**: `http://localhost:8082/docs`
- **Exercise Library**: `http://localhost:8000`
- **Coach UI**: `http://localhost:8503`
- **Grafana**: `http://localhost:3000` (User: `admin`, Pass: `admin`)
- **Prometheus**: `http://localhost:9090`
- **RAG API**: `http://localhost:8088`
- **RAG UI**: `http://localhost:8502`
- **Qdrant**: `http://localhost:6334`
- **pgAdmin**: `http://localhost:8081` via `make admin`

When `../Front-End` is present:
- **Front-End**: `http://localhost:5173`

## Common Commands

```bash
make run          # build changed images and start the whole stack
make down         # stop the whole stack
make logs         # follow logs across the stack
make ps           # show container status
make rag-ingest   # incremental RAG ingest
make rag-reingest # full RAG rebuild
make db-shell     # open psql inside postgres
make test         # go test + race detector
make test-frontend # frontend Vitest suite when ../Front-End is present
```

## Configuration

Environment variables:

- `DATABASE_URL`: Full PostgreSQL connection string
- `JWT_SECRET`: Required for authentication
- `PORT`: Default `8080`
- `APP_MODE`: `api`, `migrate`, or `worker` when running the shared binary
- `APP_ENV`: `development`, `test`, or `production`
- `GORM_LOG_LEVEL`: `silent`, `error`, `warn`, or `info`
- `CORS_ALLOWED_ORIGINS`: Comma-separated frontend origins. Defaults to common localhost frontend ports outside production; set explicitly in production, for example `https://app.example.com`
- `CORS_ALLOWED_METHODS`: Defaults to `GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS`
- `CORS_ALLOWED_HEADERS`: Optional comma-separated allowed request headers. If unset, preflights echo requested headers or use safe API defaults.
- `CORS_ALLOW_CREDENTIALS`: Defaults to `true` for explicit origins. Automatically disabled when `CORS_ALLOWED_ORIGINS=*`.
- `CORS_MAX_AGE_SECONDS`: Browser preflight cache duration. Defaults to `600`.
- `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME`, `DB_CONN_MAX_IDLE_TIME`: Database pool tuning
- `WORKER_EXPORT_POLL_INTERVAL`, `WORKER_ADMIN_REFRESH_INTERVAL`, `WORKER_NOTIFICATION_POLL_INTERVAL`: Background worker intervals for export jobs, admin view refreshes, and automated user notifications
- `PG*` (PGHOST, PGPORT, etc.): Individual connection parameters

## Seed Data Summary

Running `go run seed/main.go` generates a comprehensive dataset:

- **12 Users** (including one admin: `alex@example.com`)
- **8 Core Exercises**
- **8 Common Foods** linked to **19 Nutrients**
- **24 Workouts** with nested exercises and sets
- **12 Cardio Entries**
- **6 Workout Templates**
- **2 Training Programs**
- **36 Meals**
- **48 Weight Entries**
- **24 Favorite Foods**
- **4 Recipes**
- **12 Notifications**

All seeded users use the password: `password123`.

## API Categories

The API surface is organized into the following v1 namespaces:

- `/v1/auth`: Registration, Login, 2FA, Session management
- `/v1/users`: Profile updates, TDEE, Weight history
- `/v1/exercises`: Global exercise library
- `/v1/workouts`: Session logging, Cardio, Volume analytics
- `/v1/templates`: Workout template management
- `/v1/programs`: Multi-week training blocks
- `/v1/program-assignments`: User-side workout program assignments and session application
- `/v1/meals`: Daily food logging, Recipes, Favorite foods
- `/v1/foods`: Nutritional database lookup
- `/v1/notifications`: User alerts and reminders
- `/v1/export`: Data portability (JSON/CSV)
- `/v1/rag`: Book-query endpoints backed by the RAG service

## Testing

```bash
go test ./...
python3 test_rag_api.py --base-url http://localhost:8082
```
