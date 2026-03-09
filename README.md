# Fitness Tracker API

Lean Go backend for the core fitness tracking loop: users, exercise library, workouts, workout exercises, workout sets, meals, and weight entries.

## Scope

The backend intentionally focuses on the highest-value entities:

- `User`: profile and training context
- `Exercise`: reusable exercise library
- `Workout`: logged training session
- `WorkoutExercise`: exercise instance inside a workout
- `WorkoutSet`: set-by-set workout logging
- `Meal`: lightweight meal logging
- `WeightEntry`: simple progress tracking

Removed from the codebase:

- social and friendship features
- workout programs and enrollments
- food catalog and meal-food joins
- notifications, messages, and weekly adjustments

## Stack

- Go 1.25
- GORM
- PostgreSQL
- Standard library HTTP router

## Project Structure

```text
.
├── api/
│   ├── server.go
│   ├── helpers.go
│   ├── user_handlers.go
│   ├── exercise_handlers.go
│   ├── weight_entry_handlers.go
│   ├── workout_handlers.go
│   └── meal_handlers.go
├── database/
│   ├── database.go
│   └── migrations.go
├── models/
│   ├── user.go
│   ├── exercise.go
│   ├── weight_entry.go
│   ├── workout.go
│   ├── workout_exercise.go
│   ├── workout_set.go
│   └── meal.go
├── seed/
│   └── main.go
```

## Quick Start

```bash
make up
go run seed/main.go
go run .
```

API default address: `http://localhost:8080`

Interactive API docs:

- Swagger UI: `http://localhost:8080/docs`
- OpenAPI spec: `http://localhost:8080/openapi.yaml`

## Configuration

Environment variables:

- `DATABASE_URL`
- `PGHOST` default `localhost`
- `PGPORT` default `5433`
- `PGUSER` default `postgres`
- `PGPASSWORD` default `postgres`
- `PGDATABASE` default `fitness_tracker`
- `PGSSLMODE` default `disable`
- `PORT` default `8080`

## API Surface

### Health

- `GET /healthz`

### Users

- `POST /v1/users`
- `GET /v1/users`
- `GET /v1/users/{id}`
- `PATCH /v1/users/{id}`
- `DELETE /v1/users/{id}`

### Exercises

- `POST /v1/exercises`
- `GET /v1/exercises`
- `GET /v1/exercises/{id}`
- `PATCH /v1/exercises/{id}`
- `DELETE /v1/exercises/{id}`

### Workouts

- `POST /v1/workouts`
- `GET /v1/workouts`
- `GET /v1/users/{user_id}/workouts`
- `POST /v1/users/{user_id}/workouts`
- `GET /v1/workouts/{id}`
- `PATCH /v1/workouts/{id}`
- `DELETE /v1/workouts/{id}`

### Workout Exercises

- `POST /v1/workout-exercises`
- `GET /v1/workout-exercises`
- `GET /v1/workouts/{id}/exercises`
- `POST /v1/workouts/{id}/exercises`
- `GET /v1/workout-exercises/{id}`
- `PATCH /v1/workout-exercises/{id}`
- `DELETE /v1/workout-exercises/{id}`

### Workout Sets

- `POST /v1/workout-sets`
- `GET /v1/workout-sets`
- `GET /v1/workout-exercises/{id}/sets`
- `POST /v1/workout-exercises/{id}/sets`
- `GET /v1/workout-sets/{id}`
- `PATCH /v1/workout-sets/{id}`
- `DELETE /v1/workout-sets/{id}`

### Meals

- `POST /v1/meals`
- `GET /v1/meals`
- `GET /v1/users/{user_id}/meals`
- `POST /v1/users/{user_id}/meals`
- `GET /v1/meals/{id}`
- `PATCH /v1/meals/{id}`
- `DELETE /v1/meals/{id}`

### Weight Entries

- `POST /v1/weight-entries`
- `GET /v1/weight-entries`
- `GET /v1/users/{user_id}/weight-entries`
- `POST /v1/users/{user_id}/weight-entries`
- `GET /v1/weight-entries/{id}`
- `PATCH /v1/weight-entries/{id}`
- `DELETE /v1/weight-entries/{id}`

## Seed Data

`go run seed/main.go` creates a lean development dataset:

- 4 users
- 8 exercises
- 8 workouts with nested workout exercises and sets
- 12 meals
- 16 weight entries

## Testing

Run:

```bash
go test ./...
```
