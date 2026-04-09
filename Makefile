.PHONY: up down logs seed api migrate worker test db-shell help dev start monitor monitoring-up monitoring-down monitoring-logs monitoring-status coach restart-app restart-coach restart clean

ifneq (,$(wildcard ./.env))
    include .env
    export
endif

COMPOSE ?= docker compose
FULL_COMPOSE := $(COMPOSE) -f docker-compose.yml -f docker-compose.coach.yml
DB_SERVICES := postgres pgadmin
MONITORING_SERVICES := prometheus grafana
APP_SERVICES := migrate exercise-lib app worker
MONITORED_STACK_SERVICES := postgres pgadmin $(APP_SERVICES) prometheus grafana

# Default target
help:
	@echo "Fitness Tracker - Development Commands"
	@echo ""
	@echo "  make start           - Build everything and start the full app (one command)"
	@echo "  make up              - Start PostgreSQL, pgAdmin, and Exercise Library"
	@echo "  make down            - Stop and remove containers"
	@echo "  make migrate         - Run database migrations"
	@echo "  make seed            - Seed dummy data (requires migrated schema)"
	@echo "  make api             - Start the API server"
	@echo "  make worker          - Start the background worker"
	@echo "  make test            - Run Go tests and the race detector"
	@echo "  make db-shell        - Open psql shell in the database"
	@echo "  make logs            - Show container logs"
	@echo ""
	@echo "Monitoring Commands:"
	@echo "  make monitor         - Start the full stack with monitoring enabled"
	@echo "  make monitoring-up   - Start Prometheus and Grafana for an already running app"
	@echo "  make monitoring-down - Stop Prometheus and Grafana"
	@echo "  make monitoring-logs - Show monitoring container logs"
	@echo "  make monitoring-status - Show monitoring services status"
	@echo ""
	@echo "AI Coach Commands:"
	@echo "  make coach           - Start the Streamlit AI Coach demo (requires OPENROUTER_API_KEY)"
	@echo ""
	@echo "Restart Commands:"
	@echo "  make restart-app     - Rebuild and restart the Go API server"
	@echo "  make restart-coach   - Rebuild and restart the Streamlit AI Coach UI"
	@echo "  make restart         - Rebuild and restart both API and Coach UI"
	@echo ""
	@echo "Clean Start:"
	@echo "  make clean           - Stop and remove all containers, volumes, and images for a fresh start"
	@echo ""
	@echo "Quick Start: make start (builds everything and launches all services)"

# Docker commands
up:
	@echo "🐳 Starting containers..."
	$(COMPOSE) up -d $(DB_SERVICES) exercise-lib
	@echo ""
	@echo "✅ PostgreSQL running on localhost:5433"
	@echo "✅ pgAdmin (DB UI) running on http://localhost:8081"
	@echo "✅ Exercise Library running on http://localhost:8000"
	@echo ""
	@echo "Connection details:"
	@echo "  System: PostgreSQL"
	@echo "  Server: postgres"
	@echo "  Username: postgres"
	@echo "  Password: postgres"
	@echo "  Database: fitness_tracker"

down:
	@echo "🛑 Stopping containers..."
	$(FULL_COMPOSE) down --remove-orphans

logs:
	$(COMPOSE) logs -f

db-shell:
	docker exec -it fitness-postgres psql -U postgres -d fitness_tracker

# Application commands
migrate:
	@echo "🗃️ Running database migrations..."
	go run . migrate

seed:
	@echo "🌱 Seeding database..."
	go run seed/main.go

api:
	@echo "🚀 Starting API server..."
	go run . api

worker:
	@echo "⚙️ Starting background worker..."
	go run . worker

test:
	@echo "🧪 Running Go tests..."
	GOCACHE=/tmp/go-cache go test ./...
	@echo "🧪 Running race detector..."
	GOCACHE=/tmp/go-cache go test -race ./...

# ─── One-command start ────────────────────────────────────────────
# Builds all images, starts services in dependency order,
# waits for health checks, seeds data, and prints useful links.
start:
	@echo "🔧 Building all Docker images..."
	$(COMPOSE) build
	@echo ""
	@echo "🐳 Starting infrastructure (PostgreSQL + pgAdmin)..."
	$(COMPOSE) up -d postgres pgadmin
	@echo "⏳ Waiting for PostgreSQL to be healthy..."
	@until docker inspect --format='{{.State.Health.Status}}' fitness-postgres 2>/dev/null | grep -q healthy; do sleep 2; done
	@echo "✅ PostgreSQL is healthy"
	@echo ""
	@echo "🏋️ Starting Exercise Library..."
	$(COMPOSE) up -d exercise-lib
	@echo "⏳ Waiting for Exercise Library to be healthy (first run downloads embedding model)..."
	@until docker inspect --format='{{.State.Health.Status}}' fitness-exercise-lib 2>/dev/null | grep -q healthy; do sleep 3; done
	@echo "✅ Exercise Library is healthy"
	@echo ""
	@echo "🗃️ Running database migrations..."
	$(COMPOSE) up -d migrate
	@until docker inspect --format='{{.State.Status}}' fitness-migrate 2>/dev/null | grep -q exited; do sleep 2; done
	@docker inspect --format='{{.State.ExitCode}}' fitness-migrate | grep -q 0 && echo "✅ Migrations complete" || (echo "❌ Migrations failed" && exit 1)
	@echo ""
	@echo "🌱 Seeding database..."
	$(COMPOSE) run --rm -e EXERCISE_LIB_URL=http://exercise-lib:8000 --entrypoint /app/fitness-tracker-seed migrate
	@echo ""
	@echo "🚀 Starting API + Worker..."
	$(COMPOSE) up -d app worker
	@echo "⏳ Waiting for API server to be healthy..."
	@until docker inspect --format='{{.State.Health.Status}}' fitness-app 2>/dev/null | grep -q healthy; do sleep 2; done
	@echo "✅ API server is healthy"
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "  ✅  All services are up!"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  🌐 Application URLs:"
	@echo "     API (Swagger)      → http://localhost:8080"
	@echo "     Exercise Library   → http://localhost:8000"
	@echo "     pgAdmin (DB UI)    → http://localhost:8081"
	@echo ""
	@echo "  📡 API Endpoints:"
	@echo "     Health             → http://localhost:8080/readyz"
	@echo "     Metrics            → http://localhost:8080/metrics"
	@echo ""
	@echo "  🗄️  Database:"
	@echo "     Host: localhost  Port: 5433  DB: fitness_tracker"
	@echo "     User: postgres   Password: postgres"
	@echo ""
	@echo "  💡 Useful commands:"
	@echo "     make logs          → follow all container logs"
	@echo "     make down          → stop everything"
	@echo "     make db-shell      → open a psql shell"
	@echo ""

# Combined workflows
dev: up
	@echo "⏳ Waiting for database to be ready..."
	@sleep 3
	@echo "🗃️ Running migrations..."
	go run . migrate
	@echo "🌱 Seeding database..."
	go run seed/main.go
	@echo ""
	@echo "✅ Setup complete! Run 'make api' and 'make worker' to start the services"
	@echo "   Or 'make test' to run tests"

# Monitoring commands
monitor:
	@echo "📊 Starting full stack with monitoring..."
	$(COMPOSE) up -d $(MONITORED_STACK_SERVICES)
	@echo ""
	@echo "⏳ Waiting for services to be healthy..."
	@sleep 5
	@echo ""
	@echo "✅ All services running!"
	@echo ""
	@echo "📍 Service URLs:"
	@echo "   API:          http://localhost:8080"
	@echo "   Metrics:      http://localhost:8080/metrics"
	@echo "   Prometheus:   http://localhost:9090"
	@echo "   Grafana:      http://localhost:3000 (admin/admin)"
	@echo "   pgAdmin:      http://localhost:8081"
	@echo ""
	@echo "💡 Grafana dashboard 'Fitness Tracker - Application Metrics' is pre-configured"

monitoring-up:
	@echo "📈 Starting Prometheus and Grafana..."
	$(COMPOSE) up -d --no-deps $(MONITORING_SERVICES)
	@echo ""
	@echo "✅ Prometheus running on http://localhost:9090"
	@echo "✅ Grafana running on http://localhost:3000"
	@echo ""
	@echo "⚠️  Metrics will appear only if the app stack is already running"
	@echo "   Run 'make monitor' to start the full monitored stack"

monitoring-down:
	@echo "📉 Stopping monitoring services..."
	$(COMPOSE) stop $(MONITORING_SERVICES)
	@echo "✅ Monitoring services stopped"

monitoring-logs:
	$(COMPOSE) logs -f $(MONITORING_SERVICES)

monitoring-status:
	@echo "📊 Monitoring Services Status:"
	@echo ""
	@$(COMPOSE) ps $(MONITORING_SERVICES) app postgres 2>/dev/null || echo "   No containers running"
	@echo ""
	@echo "📍 Quick Links:"
	@echo "   Prometheus: http://localhost:9090"
	@echo "   Grafana:    http://localhost:3000"
	@echo "   Metrics:    http://localhost:8080/metrics"

# AI Coach
coach:
	@if [ -z "$(OPENROUTER_API_KEY)" ]; then \
		echo "❌ Error: OPENROUTER_API_KEY is not set in environment or .env file."; \
		echo "   Run: OPENROUTER_API_KEY=your_key make coach or add it to .env"; \
		exit 1; \
	fi
	@echo "🧠 Starting AI Coach UI and Backend..."
	@OPENROUTER_API_KEY=$(OPENROUTER_API_KEY) $(FULL_COMPOSE) up -d app coach-ui
	@echo ""
	@echo "✅ AI Coach UI is running at http://localhost:8501"
	@echo "✅ Backend is running at http://localhost:8080"

# Restart commands
restart-app:
	@echo "🔄 Rebuilding and restarting Go API server..."
	$(COMPOSE) up -d --build migrate app worker
	@echo "⏳ Waiting for health check..."
	@sleep 3
	@curl -sf http://localhost:8080/readyz > /dev/null && echo "✅ API server is healthy at http://localhost:8080" || echo "⚠️  API server not ready yet, check 'docker logs fitness-app'"

restart-coach:
	@echo "🔄 Rebuilding and restarting Streamlit AI Coach UI..."
	$(FULL_COMPOSE) up -d --build coach-ui
	@echo "✅ Coach UI restarted at http://localhost:8501"

# Clean start - remove all containers, volumes, and images
clean:
	@echo "🧹 Cleaning all containers, volumes, and images..."
	$(FULL_COMPOSE) down --remove-orphans --volumes --rmi local
	@echo "✅ Clean complete! Run 'make dev' or 'make up && make migrate' for a fresh start."

restart: restart-app
	@echo "🔄 Rebuilding and restarting Streamlit AI Coach UI..."
	$(FULL_COMPOSE) up -d --build coach-ui
	@echo ""
	@echo "✅ All services restarted!"
	@echo "   API:      http://localhost:8080"
	@echo "   Coach UI: http://localhost:8501"
