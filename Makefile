.PHONY: up down logs seed api test db-shell help dev monitor monitoring-up monitoring-down monitoring-logs monitoring-status

COMPOSE ?= docker compose
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
	$(COMPOSE) down

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
