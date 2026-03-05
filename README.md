# Fitness Tracker API

A comprehensive fitness tracking backend built with Go and GORM, featuring workout logging, nutrition tracking, weight management, social features, and workout program enrollment.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Project Structure](#project-structure)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Configuration](#configuration)
- [API Reference](#api-reference)
- [Data Models](#data-models)
- [Development](#development)
- [Testing](#testing)
- [Docker Setup](#docker-setup)

## Overview

This project is a RESTful API backend for a fitness tracking application. It provides endpoints for managing users, workouts, meals, weight entries, friendships, and workout programs. Built with Go 1.25 and GORM ORM with PostgreSQL as the database.

## Features

### Core Features

- **User Management**: User profiles with fitness metrics (weight, height, TDEE, goals)
- **Workout Logging**: Track workouts with exercises, sets, reps, weight, and RPE
- **Set-by-Set Tracking**: Granular workout logging with individual set details
- **Nutrition Tracking**: Log meals with food items and quantities
- **Weight Tracking**: Monitor weight progress over time
- **Social Features**: Friend requests and friendships between users
- **Workout Programs**: Structured multi-week programs with progress tracking
- **AI Integration Ready**: Weekly TDEE adjustments with AI reasoning storage

### Technical Features

- RESTful API design
- PostgreSQL database with UUID primary keys
- Soft deletes for all entities
- Database migrations with backward compatibility
- Comprehensive seed data for development
- Docker development environment

## Project Structure

```
.
├── main.go                 # Application entry point
├── go.mod                  # Go module definition
├── go.sum                  # Dependency checksums
├── Makefile                # Development commands
├── docker-compose.yml      # Docker services configuration
├── .env.example            # Environment variables template
│
├── api/                    # HTTP handlers and routing
│   ├── server.go           # Server setup and route registration
│   ├── workout_handlers.go # Workout and exercise endpoints
│   ├── meal_handlers.go    # Meal tracking endpoints
│   ├── friendship_handlers.go # Social features endpoints
│   └── program_handlers.go # Workout program endpoints
│
├── database/               # Database connection and migrations
│   ├── database.go         # PostgreSQL connection setup
│   └── migrations.go       # Schema migrations
│
├── models/                 # GORM data models
│   ├── user.go             # User entity
│   ├── workout.go          # Workout session
│   ├── exercise.go         # Exercise library
│   ├── workout_exercise.go # Workout-Exercise join table
│   ├── workout_set.go      # Set-by-set logging
│   ├── workout_program.go  # Multi-week programs
│   ├── program_enrollment.go # User program enrollment
│   ├── program_progress.go # Daily progress tracking
│   ├── meal.go             # Meal logging
│   ├── food.go             # Food database
│   ├── meal_food.go        # Meal-Food join table
│   ├── weight_entry.go     # Weight tracking
│   ├── friendship.go       # User connections
│   ├── message.go          # Direct messaging
│   ├── notification.go     # User notifications
│   └── weekly_adjustment.go # TDEE adjustments
│
├── seed/                   # Database seeding
│   └── main.go             # Seed script with sample data
│
└── tests/                  # Test files
    ├── test_model_hooks.py
    └── test_schema_contracts.py
```

## Quick Start

The fastest way to get started:

```bash
# Clone the repository
git clone <repository-url>
cd fitness-tracker

# Start PostgreSQL and pgAdmin
make up

# Run migrations and seed sample data
make seed

# Start the API server
make api
```

The API will be available at `http://localhost:8080`.

## Installation

### Prerequisites

- Go 1.25 or later
- PostgreSQL 16 (or use Docker)
- Make (optional, for convenience commands)

### Manual Setup

1. **Install Go dependencies**:
   ```bash
   go mod download
   ```

2. **Set up PostgreSQL**:
   - Install PostgreSQL or use Docker (see [Docker Setup](#docker-setup))
   - Create a database named `fitness_tracker`

3. **Configure environment**:
   ```bash
   cp .env.example .env
   ```
   Edit `.env` with your database credentials.

4. **Run migrations and seed**:
   ```bash
   go run seed/main.go
   ```

5. **Start the server**:
   ```bash
   go run main.go
   ```

## Configuration

Configuration is handled through environment variables. You can set them in a `.env` file or directly in your environment.

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | Full PostgreSQL connection string | - |
| `PGHOST` | Database host | `localhost` |
| `PGPORT` | Database port | `5433` |
| `PGUSER` | Database user | `postgres` |
| `PGPASSWORD` | Database password | `postgres` |
| `PGDATABASE` | Database name | `fitness_tracker` |
| `PGSSLMODE` | SSL mode | `disable` |
| `PORT` | API server port | `8080` |

### Example .env File

```env
# Using DATABASE_URL (recommended)
DATABASE_URL=postgres://postgres:postgres@localhost:5433/fitness_tracker?sslmode=disable

# Or use individual variables
PGHOST=localhost
PGPORT=5433
PGUSER=postgres
PGPASSWORD=postgres
PGDATABASE=fitness_tracker
PGSSLMODE=disable

# Server
PORT=8080
```

## API Reference

### Health Check

```
GET /healthz
```

Returns server health status.

### Workouts

```
POST /v1/workouts              # Create a new workout
GET  /v1/workouts/{id}         # Get workout details
POST /v1/workouts/{id}/exercises    # Add exercise to workout
GET  /v1/workout-exercises/{id}/sets    # List sets for an exercise
POST /v1/workout-exercises/{id}/sets    # Create a new set
PATCH /v1/workout-sets/{id}    # Update a set
DELETE /v1/workout-sets/{id}   # Delete a set
```

#### Create Workout Example

```bash
curl -X POST http://localhost:8080/v1/workouts \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-uuid-here",
    "date": "2024-03-04",
    "duration": 60,
    "type": "push",
    "notes": "Great workout!",
    "exercises": [
      {
        "exercise_id": "exercise-uuid-here",
        "order": 1,
        "sets": 3,
        "reps": 10,
        "weight": 60.5,
        "set_entries": [
          {"set_number": 1, "reps": 10, "weight": 60, "rpe": 8},
          {"set_number": 2, "reps": 10, "weight": 60, "rpe": 8.5},
          {"set_number": 3, "reps": 9, "weight": 60, "rpe": 9}
        ]
      }
    ]
  }'
```

### Meals

```
POST /v1/meals                      # Create a meal
GET  /v1/users/{user_id}/meals      # List user's meals
PATCH /v1/meals/{id}                # Update a meal
DELETE /v1/meals/{id}               # Delete a meal
```

#### Query Parameters for Listing Meals

- `date`: Filter by date (YYYY-MM-DD format)
- `meal_type`: Filter by meal type (breakfast, lunch, dinner, snack)

#### Create Meal Example

```bash
curl -X POST http://localhost:8080/v1/meals \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-uuid-here",
    "meal_type": "breakfast",
    "date": "2024-03-04",
    "notes": "Morning meal"
  }'
```

### Friendships

```
POST /v1/friendships/requests                    # Send friend request
GET  /v1/users/{user_id}/friendships/incoming    # List incoming requests
GET  /v1/users/{user_id}/friendships/outgoing    # List outgoing requests
GET  /v1/users/{user_id}/friends                 # List accepted friends
PATCH /v1/friendships/{id}/accept                # Accept friend request
DELETE /v1/friendships/{id}                      # Delete/reject friendship
```

#### Send Friend Request Example

```bash
curl -X POST http://localhost:8080/v1/friendships/requests \
  -H "Content-Type: application/json" \
  -d '{
    "requester_id": "user-uuid-here",
    "addressee_id": "target-user-uuid"
  }'
```

#### Accept Friend Request Example

```bash
curl -X PATCH http://localhost:8080/v1/friendships/{friendship-id}/accept \
  -H "Content-Type: application/json" \
  -d '{
    "actor_user_id": "accepting-user-uuid"
  }'
```

### Workout Programs

```
POST /v1/program-enrollments                         # Enroll in a program
GET  /v1/users/{user_id}/program-enrollments         # List user enrollments
GET  /v1/program-enrollments/{id}                    # Get enrollment details
PATCH /v1/program-enrollments/{id}                   # Update enrollment
POST /v1/program-enrollments/{id}/progress           # Log progress
GET  /v1/program-enrollments/{id}/progress           # List progress
PATCH /v1/program-progress/{id}                      # Update progress entry
```

#### Enroll in Program Example

```bash
curl -X POST http://localhost:8080/v1/program-enrollments \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-uuid-here",
    "workout_program_id": "program-uuid-here",
    "status": "active"
  }'
```

## Data Models

### Entity Relationship Diagram

```
User
  ├── Workout (1:N)
  │     └── WorkoutExercise (1:N)
  │           └── WorkoutSet (1:N)
  │
  ├── Meal (1:N)
  │     └── MealFood (1:N)
  │           └── Food (N:1)
  │
  ├── WeightEntry (1:N)
  ├── Friendship (1:N)
  ├── Message (1:N sent/received)
  ├── Notification (1:N)
  ├── WeeklyAdjustment (1:N)
  ├── WorkoutProgram (1:N created)
  └── ProgramEnrollment (1:N)
        └── ProgramProgress (1:N)
```

### Key Models

#### User

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Primary key |
| `email` | string | Unique email address |
| `name` | string | Display name |
| `weight` | decimal | Current weight (kg) |
| `height` | decimal | Height (cm) |
| `goal` | string | Fitness goal |
| `activity_level` | string | Activity level |
| `tdee` | int | Total Daily Energy Expenditure |

#### Workout

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Primary key |
| `user_id` | UUID | Owner reference |
| `date` | date | Workout date |
| `duration` | int | Duration in minutes |
| `type` | string | Workout type (push/pull/legs/cardio) |
| `notes` | text | Optional notes |

#### WorkoutSet

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Primary key |
| `workout_exercise_id` | UUID | Parent exercise reference |
| `set_number` | int | Set position |
| `reps` | int | Repetitions |
| `weight` | decimal | Weight used |
| `rpe` | decimal | Rate of Perceived Exertion |
| `completed` | bool | Completion status |

## Development

### Available Make Commands

```bash
make help      # Show all available commands
make up        # Start Docker containers (PostgreSQL + pgAdmin)
make down      # Stop and remove containers
make seed      # Run migrations and seed database
make api       # Start the API server
make logs      # View container logs
make db-shell  # Open psql shell
make dev       # Full setup: up + seed
```

### Database Migrations

Migrations are handled automatically by GORM's AutoMigrate. The application also includes manual migration functions for complex schema changes (see `database/migrations.go`).

### Adding New Models

1. Create a new file in `models/` directory
2. Define the struct with GORM tags
3. Add `BeforeCreate` hook for UUID generation
4. Register the model in `database/migrations.go`
5. Create corresponding handlers in `api/`

## Testing

### Running Tests

The project includes Python-based tests for model validation:

```bash
# Ensure containers are running
make up

# Run tests
python -m pytest tests/
```

### Seed Data

The seed script creates comprehensive sample data:

| Entity | Count |
|--------|-------|
| Exercises | 20 |
| Foods | 30 |
| Users | 10 |
| Workouts | ~30 |
| Meals | ~50 |
| Weight Entries | ~40 |
| Friendships | ~15 |
| Workout Programs | 3 |

## Docker Setup

### Services

The `docker-compose.yml` defines two services:

| Service | Port | Description |
|---------|------|-------------|
| PostgreSQL | 5433 | Main database |
| pgAdmin4 | 8081 | Database management UI |

### Starting Services

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down

# Reset everything (including data)
docker-compose down -v
```

### pgAdmin4 Access

1. Open http://localhost:8081
2. Login credentials:
   - Email: `admin@fitness-tracker.com`
   - Password: `admin`
3. Add server connection:
   - Host: `fitness-postgres` (or `postgres` from within Docker)
   - Port: `5432` (internal Docker port)
   - Username: `postgres`
   - Password: `postgres`

### Database Connection Details

When connecting from the host machine:

```
Host: localhost
Port: 5433
User: postgres
Password: postgres
Database: fitness_tracker
```

## API Response Format

### Success Response

```json
{
  "id": "uuid",
  "field": "value",
  ...
}
```

### Error Response

```json
{
  "error": "Error message description"
}
```

### HTTP Status Codes

| Code | Description |
|------|-------------|
| 200 | Success |
| 201 | Created |
| 204 | No Content (successful deletion) |
| 400 | Bad Request |
| 404 | Not Found |
| 409 | Conflict |
| 500 | Internal Server Error |

## Dependencies

| Package | Purpose |
|---------|---------|
| `gorm.io/gorm` | ORM for database operations |
| `gorm.io/driver/postgres` | PostgreSQL driver for GORM |
| `github.com/google/uuid` | UUID generation |
| `github.com/joho/godotenv` | Environment variable loading |
| `gorm.io/datatypes` | JSON/JSONB support |

## License

This project is provided as-is for educational and development purposes.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and ensure they pass
5. Submit a pull request