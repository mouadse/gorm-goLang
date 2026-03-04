# Fitness Tracker - Docker Development Setup

This guide explains how to run the Fitness Tracker API with PostgreSQL and pgAdmin4 using Docker.

## Quick Start

```bash
# 1. Start the containers (PostgreSQL + pgAdmin4)
make up

# 2. Seed the database with dummy data
make seed

# 3. Start the API server
make api
```

## Services

| Service | URL | Description |
|---------|-----|-------------|
| PostgreSQL | `localhost:5433` | Main database |
| pgAdmin4 | http://localhost:8081 | Database management UI |
| API | http://localhost:8080 | Fitness Tracker API |

## Database Connection Details

When connecting via pgAdmin4 or other tools:

- **System**: PostgreSQL
- **Server**: `localhost` (or `fitness-postgres` from within containers)
- **Port**: `5433`
- **Username**: `postgres`
- **Password**: `postgres`
- **Database**: `fitness_tracker`

## Available Make Commands

```bash
make up       # Start PostgreSQL and pgAdmin4 containers
make down     # Stop and remove containers
make seed     # Run database migrations and seed dummy data
make api      # Start the API server
make logs     # Show container logs
make db-shell # Open psql shell in the database
make dev      # Full setup: up + seed
```

## Manual Setup (without Make)

```bash
# Start containers
docker-compose up -d

# Copy environment file
cp .env.example .env

# Run migrations and seed
go run seed/main.go

# Start API
go run main.go
```

## Seeded Data

The seed script creates:

- **20 Exercises** - Bench Press, Squat, Deadlift, etc.
- **30 Foods** - Chicken, Rice, Salmon, etc. with macros
- **10 Users** - With varied goals and profiles
- **~30 Workouts** - With exercises and sets
- **~50 Meals** - With food entries
- **~40 Weight Entries** - Historical weight tracking
- **~15 Friendships** - Pending and accepted
- **3 Workout Programs** - With enrollments and progress

## Testing the API

After starting the server, test with curl:

```bash
# Health check
curl http://localhost:8080/healthz

# List users via database query or create API calls

# Create a workout
curl -X POST http://localhost:8080/v1/workouts \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "<user-uuid-from-db>",
    "date": "2024-03-04",
    "duration": 60,
    "type": "push",
    "notes": "Great workout!",
    "exercises": [
      {
        "exercise_id": "<exercise-uuid-from-db>",
        "sets": 3,
        "reps": 10,
        "weight": 60
      }
    ]
  }'
```

## Viewing Data in pgAdmin4

1. Open http://localhost:8081
2. Login with:
   - **Email**: `admin@fitness-tracker.com`
   - **Password**: `admin`
3. Add a new server connection:
   - Right-click "Servers" → "Register" → "Server"
   - **Name**: `Fitness Tracker` (any name you want)
   - **Connection tab**:
     - **Host**: `fitness-postgres`
     - **Port**: `5432`
     - **Username**: `postgres`
     - **Password**: `postgres`
4. Browse tables: `users`, `workouts`, `meals`, etc.
5. Run SQL queries via "Query Tool"

## Stopping Everything

```bash
make down
```

This stops and removes the containers but keeps the data in the Docker volume.

## Reset Database

To completely reset (remove all data):

```bash
docker-compose down -v  # Remove containers AND volumes
make up                 # Start fresh
make seed               # Re-seed data
```

## Troubleshooting

### Port already in use
If port 5433 is taken, edit `docker-compose.yml` and change the port mapping:
```yaml
ports:
  - "5434:5432"  # Use 5434 instead
```
Then update `.env`:
```
DATABASE_URL=postgres://postgres:postgres@localhost:5434/fitness_tracker?sslmode=disable
```

### Connection refused
Wait a few seconds after `make up` for PostgreSQL to fully start before running `make seed`.

### Check container status
```bash
docker-compose ps
docker-compose logs postgres
```
