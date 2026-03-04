.PHONY: up down seed api test db-shell help

# Default target
help:
	@echo "Fitness Tracker - Development Commands"
	@echo ""
	@echo "  make up       - Start PostgreSQL and phpMyAdmin containers"
	@echo "  make down     - Stop and remove containers"
	@echo "  make seed     - Run database migrations and seed dummy data"
	@echo "  make api      - Start the API server"
	@echo "  make test     - Run API tests (requires containers to be running)"
	@echo "  make db-shell - Open psql shell in the database"
	@echo "  make logs     - Show container logs"
	@echo ""
	@echo "Quick Start: make up && make seed && make api"

# Docker commands
up:
	@echo "🐳 Starting containers..."
	docker-compose up -d
	@echo ""
	@echo "✅ PostgreSQL running on localhost:5433"
	@echo "✅ Adminer (DB UI) running on http://localhost:8081"
	@echo ""
	@echo "Connection details:"
	@echo "  System: PostgreSQL"
	@echo "  Server: postgres"
	@echo "  Username: postgres"
	@echo "  Password: postgres"
	@echo "  Database: fitness_tracker"

down:
	@echo "🛑 Stopping containers..."
	docker-compose down

logs:
	docker-compose logs -f

db-shell:
	docker exec -it fitness-postgres psql -U postgres -d fitness_tracker

# Application commands
seed:
	@echo "🌱 Seeding database..."
	go run seed/main.go

api:
	@echo "🚀 Starting API server..."
	go run main.go

# Combined workflows
dev: up
	@echo "⏳ Waiting for database to be ready..."
	@sleep 3
	@echo "🌱 Seeding database..."
	go run seed/main.go
	@echo ""
	@echo "✅ Setup complete! Run 'make api' to start the server"
	@echo "   Or 'make test' to run tests"
