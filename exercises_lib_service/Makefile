.PHONY: help up down ps logs config build

ENV_FILE ?= ../.env
COMPOSE ?= docker compose --env-file $(ENV_FILE)

help:
	@echo "Available commands:"
	@echo "  make up      - Start the exercise library service"
	@echo "  make down    - Stop the exercise library service"
	@echo "  make ps      - Show compose service status"
	@echo "  make logs    - Tail service logs"
	@echo "  make config  - Render resolved compose config"
	@echo "  make build   - Build the runtime image"

up:
	$(COMPOSE) up -d

down:
	$(COMPOSE) down

ps:
	$(COMPOSE) ps

logs:
	$(COMPOSE) logs -f

config:
	$(COMPOSE) config

build:
	$(COMPOSE) build
