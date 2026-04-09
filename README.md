# Fitness Tracker API

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/mouadse/gorm-goLang)

Comprehensive Go backend for a complete fitness tracking ecosystem: users, exercises, workouts, nutrition, analytics, and automation.

## Features & Scope

The backend provides a robust API for full-spectrum fitness tracking:

- **User Management**: Profiles, goals, TDEE calculation, and two-factor authentication (2FA).
- **Exercise Library**: Extensive muscle-group focused exercise database with instructions and video URLs.
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
├── scripts/            # Utility scripts (USDA data import)
└── seed/               # Idempotent development data seeder
```

## Quick Start

### 1. Start Infrastructure
```bash
cp .env.example .env
docker compose up -d postgres pgadmin exercise-lib
```

### 2. Run Migrations
```bash
go run . migrate
```

### 3. Seed Database
```bash
go run seed/main.go
```

### 4. Run Application and Worker
```bash
export JWT_SECRET=your-secure-random-secret
go run . api
go run . worker
```

API default address: `http://localhost:8080`

## Monitoring & Management

- **Swagger UI**: `http://localhost:8080/docs`
- **Grafana**: `http://localhost:3000` (User: `admin`, Pass: `admin`)
- **Prometheus**: `http://localhost:9090`
- **pgAdmin**: `http://localhost:8081` (User: `admin@fitness-tracker.com`, Pass: `admin`)

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
- `/v1/meals`: Daily food logging, Recipes, Favorite foods
- `/v1/foods`: Nutritional database lookup
- `/v1/notifications`: User alerts and reminders
- `/v1/export`: Data portability (JSON/CSV)

## Testing

```bash
go test ./...
```
