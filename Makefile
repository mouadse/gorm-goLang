.PHONY: run down logs test db-shell clean help

ifneq (,$(wildcard ./.env))
    include .env
    export
endif

COMPOSE ?= docker compose
FULL_COMPOSE := $(COMPOSE) -f docker-compose.yml -f docker-compose.coach.yml

# Core services always started by `make run`
CORE_SERVICES := postgres pgadmin exercise-lib migrate seed app worker

# ─── One command to rule them all ──────────────────────────────────
# Builds images and starts services in the correct order.
# Docker Compose dependency chains handle health-checks automatically:
#   postgres → migrate → seed → (app + worker) → [prometheus → grafana]
#   exercise-lib ────────────↗
#
# Optional flags:
#   MONITOR=1   also start Prometheus + Grafana
#   COACH=1     also start the Streamlit AI Coach UI
#   Examples:  make run
#              make run MONITOR=1
#              make run COACH=1
#              make run MONITOR=1 COACH=1
run:
	@echo "🔧 Building images..."
	$(COMPOSE) build
ifdef MONITOR
	@echo "📊 Including monitoring (Prometheus + Grafana)..."
endif
ifdef COACH
	@echo "🧠 Including AI Coach UI..."
endif
	@echo "🐳 Starting services..."
	$(COMPOSE) up -d $(CORE_SERVICES) $(if $(MONITOR),prometheus grafana,)
ifdef COACH
	$(FULL_COMPOSE) up -d coach-ui
endif
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "  ✅  All services are up!"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  🌐 Application URLs:"
	@echo "     API (Swagger)      → http://localhost:8080"
	@echo "     Exercise Library   → http://localhost:8000"
	@echo "     pgAdmin (DB UI)    → http://localhost:8081"
ifdef MONITOR
	@echo "     Prometheus         → http://localhost:9090"
	@echo "     Grafana            → http://localhost:3000 (admin/admin)"
endif
ifdef COACH
	@echo "     AI Coach UI        → http://localhost:8501"
endif
	@echo ""
	@echo "  🗄️  Database:"
	@echo "     Host: localhost  Port: 5433  DB: fitness_tracker"
	@echo "     User: postgres   Password: postgres"
	@echo ""
	@echo "  💡 Useful commands:"
	@echo "     make logs          → follow container logs"
	@echo "     make down          → stop everything"
	@echo "     make db-shell      → open a psql shell"
	@echo "     make clean         → remove everything for a fresh start"
	@echo ""

# ─── Lifecycle ─────────────────────────────────────────────────────
down:
	@echo "🛑 Stopping containers..."
	$(FULL_COMPOSE) down --remove-orphans

clean:
	@echo "🧹 Removing all containers, volumes, and built images..."
	$(FULL_COMPOSE) down --remove-orphans --volumes --rmi local
	@echo "✅ Clean complete. Run 'make run' for a fresh start."

# ─── Utilities ─────────────────────────────────────────────────────
logs:
	$(COMPOSE) logs -f

db-shell:
	docker exec -it fitness-postgres psql -U postgres -d fitness_tracker

test:
	@echo "🧪 Running Go tests..."
	GOCACHE=/tmp/go-cache go test ./...
	@echo "🧪 Running race detector..."
	GOCACHE=/tmp/go-cache go test -race ./...

# ─── Help ──────────────────────────────────────────────────────────
help:
	@echo "Fitness Tracker — Available Commands"
	@echo ""
	@echo "  make run              Build & start all core services"
	@echo "  make run MONITOR=1    Same, plus Prometheus + Grafana"
	@echo "  make run COACH=1      Same, plus Streamlit AI Coach UI"
	@echo "  make down             Stop and remove containers"
	@echo "  make clean            Stop, remove containers/volumes/images"
	@echo "  make logs             Follow container logs"
	@echo "  make test             Run Go tests + race detector"
	@echo "  make db-shell         Open psql shell"
	@echo ""