.PHONY: up down logs seed api test db-shell help dev monitor monitoring-up monitoring-down monitoring-logs monitoring-status coach restart-app restart-coach restart

ifneq (,$(wildcard ./.env))
    include .env
    export
endif

COMPOSE ?= docker compose
FULL_COMPOSE := $(COMPOSE) -f docker-compose.yml -f docker-compose.coach.yml
DB_SERVICES := postgres pgadmin
MONITORING_SERVICES := prometheus grafana
MONITORED_STACK_SERVICES := postgres pgadmin app prometheus grafana

# Default target
help:
	@echo "Fitness Tracker - Development Commands"
	@echo ""
	@echo "  make up              - Start PostgreSQL and pgAdmin containers"
	@echo "  make down            - Stop and remove containers"
	@echo "  make seed            - Run database migrations and seed dummy data"
	@echo "  make api             - Start the API server"
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
	@echo "Quick Start: make up && make seed && make api"
	@echo "With Monitoring: make monitor"

# Docker commands
up:
	@echo "🐳 Starting containers..."
	$(COMPOSE) up -d $(DB_SERVICES)
	@echo ""
	@echo "✅ PostgreSQL running on localhost:5433"
	@echo "✅ pgAdmin (DB UI) running on http://localhost:8081"
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
seed:
	@echo "🌱 Seeding database..."
	go run seed/main.go

api:
	@echo "🚀 Starting API server..."
	go run main.go

test:
	@echo "🧪 Running Go tests..."
	GOCACHE=/tmp/go-cache go test ./...
	@echo "🧪 Running race detector..."
	GOCACHE=/tmp/go-cache go test -race ./...

# Combined workflows
dev: up
	@echo "⏳ Waiting for database to be ready..."
	@sleep 3
	@echo "🌱 Seeding database..."
	go run seed/main.go
	@echo ""
	@echo "✅ Setup complete! Run 'make api' to start the server"
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
	$(COMPOSE) up -d --build app
	@echo "⏳ Waiting for health check..."
	@sleep 3
	@curl -sf http://localhost:8080/healthz > /dev/null && echo "✅ API server is healthy at http://localhost:8080" || echo "⚠️  API server not ready yet, check 'docker logs fitness-app'"

restart-coach:
	@echo "🔄 Rebuilding and restarting Streamlit AI Coach UI..."
	$(FULL_COMPOSE) up -d --build coach-ui
	@echo "✅ Coach UI restarted at http://localhost:8501"

restart: restart-app
	@echo "🔄 Rebuilding and restarting Streamlit AI Coach UI..."
	$(FULL_COMPOSE) up -d --build coach-ui
	@echo ""
	@echo "✅ All services restarted!"
	@echo "   API:      http://localhost:8080"
	@echo "   Coach UI: http://localhost:8501"
