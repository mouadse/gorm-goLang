.PHONY: help run up down restart logs ps config build clean test db-shell admin rag-ingest rag-reingest rag-shell

ifneq (,$(wildcard ./.env))
include .env
export
endif

API_HOST_PORT ?= 8082
EXERCISE_LIB_HOST_PORT ?= 8000
COACH_UI_HOST_PORT ?= 8503
GRAFANA_HOST_PORT ?= 3000
PROMETHEUS_HOST_PORT ?= 9090
PGADMIN_HOST_PORT ?= 8081
POSTGRES_HOST_PORT ?= 5433
WORKER_METRICS_HOST_PORT ?= 9091
RAG_API_PORT ?= 8088
RAG_UI_PORT ?= 8502
RAG_QDRANT_PORT ?= 6334

COMPOSE = DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker compose \
	-f docker-compose.yml \
	-f docker-compose.coach.yml \
	-f docker-compose.rag.yml

BUILD_SERVICES = app exercise-lib coach-ui rag-api rag-ui
STACK_SERVICES = postgres exercise-lib migrate seed app worker prometheus grafana coach-ui rag-qdrant rag-api rag-ui

help:
	@echo "Available commands:"
	@echo "  make run          Build what changed and start the full dev stack"
	@echo "  make down         Stop the stack"
	@echo "  make restart      Restart the running stack"
	@echo "  make logs         Follow logs for the full stack"
	@echo "  make ps           Show service status"
	@echo "  make config       Render the merged compose config"
	@echo "  make admin        Start pgAdmin only when you need it"
	@echo "  make rag-ingest   Run incremental RAG ingestion"
	@echo "  make rag-reingest Run a full RAG reindex"
	@echo "  make rag-shell    Open a shell in the RAG ingest image"
	@echo "  make db-shell     Open psql in the postgres container"
	@echo "  make test         Run Go tests and the race detector"
	@echo "  make clean        Remove containers, volumes, and local images"

run:
	@echo "Building application images..."
	$(COMPOSE) build --parallel $(BUILD_SERVICES)
	@echo "Starting the full stack..."
	$(COMPOSE) up -d --remove-orphans $(STACK_SERVICES)
	@echo ""
	@echo "Full stack endpoints:"
	@echo "  API              http://localhost:$(API_HOST_PORT)"
	@echo "  Exercise Library http://localhost:$(EXERCISE_LIB_HOST_PORT)"
	@echo "  Coach UI         http://localhost:$(COACH_UI_HOST_PORT)"
	@echo "  Grafana          http://localhost:$(GRAFANA_HOST_PORT)"
	@echo "  Prometheus       http://localhost:$(PROMETHEUS_HOST_PORT)"
	@echo "  RAG API          http://localhost:$(RAG_API_PORT)"
	@echo "  RAG UI           http://localhost:$(RAG_UI_PORT)"
	@echo "  Qdrant           http://localhost:$(RAG_QDRANT_PORT)"
	@echo ""
	@echo "Initial RAG bootstrap runs automatically from rag_setup/books."
	@echo "Use 'make rag-ingest' after adding or changing PDFs while the stack is already running."

up: run

down:
	$(COMPOSE) down --remove-orphans

restart:
	$(COMPOSE) restart $(STACK_SERVICES)

logs:
	$(COMPOSE) logs -f --tail=200 $(STACK_SERVICES)

ps:
	$(COMPOSE) ps

config:
	$(COMPOSE) config

build:
	$(COMPOSE) build --parallel $(BUILD_SERVICES)

admin:
	$(COMPOSE) up -d pgadmin
	@echo "pgAdmin: http://localhost:8081"

db-shell:
	$(COMPOSE) exec postgres psql -U postgres -d fitness_tracker

rag-ingest:
	$(COMPOSE) --profile tools run --rm rag-ingest python ingest.py
	@if [ -n "$$($(COMPOSE) ps -q rag-api)" ]; then $(COMPOSE) restart rag-api; fi

rag-reingest:
	$(COMPOSE) --profile tools run --rm rag-ingest python ingest.py --reset
	@if [ -n "$$($(COMPOSE) ps -q rag-api)" ]; then $(COMPOSE) restart rag-api; fi

rag-shell:
	$(COMPOSE) --profile tools run --rm rag-ingest sh

test:
	GOCACHE=/tmp/go-cache go test ./...
	GOCACHE=/tmp/go-cache go test -race ./...

clean:
	$(COMPOSE) down --remove-orphans --volumes --rmi local
