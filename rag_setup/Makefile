.PHONY: \
	help ps config down prune \
	up up-core up-api up-ui up-qdrant \
	build build-api build-ingest build-ui rebuild rebuild-api rebuild-ingest rebuild-ui \
	restart restart-api restart-ui restart-qdrant \
	logs logs-api logs-ui logs-qdrant \
	shell-api shell-ui shell-qdrant \
	exec-api exec-ui exec-qdrant \
	ingest reingest ingest-shell

ENV_FILE ?= ../.env
COMPOSE ?= docker compose --env-file $(ENV_FILE)
TORCH_BACKEND ?= cpu
TORCH_BUILD_ARG := --build-arg TORCH_BACKEND=$(TORCH_BACKEND)

help:
	@echo "Available commands:"
	@echo "  make up            - Start qdrant, api, and ui"
	@echo "  make up-core       - Start qdrant and api only"
	@echo "  make up-api        - Start/restart api only"
	@echo "  make up-ui         - Start/restart ui only"
	@echo "  make up-qdrant     - Start/restart qdrant only"
	@echo "  make down          - Stop all containers and keep volumes"
	@echo "  make prune         - Stop containers and remove orphaned one-off containers"
	@echo "  make ps            - Show compose service status"
	@echo "  make config        - Render resolved compose config"
	@echo "  make build         - Build api, ingest, and ui images"
	@echo "  make build-api     - Build api image"
	@echo "  make build-ingest  - Build ingest image"
	@echo "  make build-ui      - Build ui image"
	@echo "  make rebuild       - Rebuild api, ingest, and ui without cache"
	@echo "  make rebuild-api   - Rebuild api without cache"
	@echo "  make rebuild-ingest - Rebuild ingest without cache"
	@echo "  make rebuild-ui    - Rebuild ui without cache"
	@echo "  make restart       - Restart qdrant, api, and ui"
	@echo "  make restart-api   - Restart api"
	@echo "  make restart-ui    - Restart ui"
	@echo "  make restart-qdrant - Restart qdrant"
	@echo "  make logs          - Tail logs for qdrant, api, and ui"
	@echo "  make logs-api      - Tail api logs"
	@echo "  make logs-ui       - Tail ui logs"
	@echo "  make logs-qdrant   - Tail qdrant logs"
	@echo "  make shell-api     - Open a shell inside api"
	@echo "  make shell-ui      - Open a shell inside ui"
	@echo "  make shell-qdrant  - Open a shell inside qdrant"
	@echo "  make exec-api CMD='python -m pytest'"
	@echo "  make exec-ui CMD='python ui.py'"
	@echo "  make exec-qdrant CMD='ls /qdrant/storage'"
	@echo "  make ingest        - Run incremental ingest job"
	@echo "  make reingest      - Run full ingest reset"
	@echo "  make ingest-shell  - Open a shell in a one-off ingest container"
	@echo "  TORCH_BACKEND=cpu|auto|<cuda> (default: cpu)"

ps:
	$(COMPOSE) ps

config:
	$(COMPOSE) config

up:
	$(COMPOSE) up -d qdrant api ui

up-core:
	$(COMPOSE) up -d qdrant api

up-api:
	$(COMPOSE) up -d api

up-ui:
	$(COMPOSE) up -d ui

up-qdrant:
	$(COMPOSE) up -d qdrant

down:
	$(COMPOSE) down

prune:
	$(COMPOSE) down --remove-orphans

build:
	$(COMPOSE) build $(TORCH_BUILD_ARG) api ingest
	$(COMPOSE) build ui

build-api:
	$(COMPOSE) build $(TORCH_BUILD_ARG) api

build-ingest:
	$(COMPOSE) build $(TORCH_BUILD_ARG) ingest

build-ui:
	$(COMPOSE) build ui

rebuild:
	$(COMPOSE) build --no-cache $(TORCH_BUILD_ARG) api ingest
	$(COMPOSE) build --no-cache ui

rebuild-api:
	$(COMPOSE) build --no-cache $(TORCH_BUILD_ARG) api

rebuild-ingest:
	$(COMPOSE) build --no-cache $(TORCH_BUILD_ARG) ingest

rebuild-ui:
	$(COMPOSE) build --no-cache ui

restart:
	$(COMPOSE) restart qdrant api ui

restart-api:
	$(COMPOSE) restart api

restart-ui:
	$(COMPOSE) restart ui

restart-qdrant:
	$(COMPOSE) restart qdrant

logs:
	$(COMPOSE) logs -f qdrant api ui

logs-api:
	$(COMPOSE) logs -f api

logs-ui:
	$(COMPOSE) logs -f ui

logs-qdrant:
	$(COMPOSE) logs -f qdrant

shell-api:
	$(COMPOSE) exec api sh

shell-ui:
	$(COMPOSE) exec ui sh

shell-qdrant:
	$(COMPOSE) exec qdrant sh

exec-api:
	@test -n "$(CMD)" || (echo "Usage: make exec-api CMD='...'" && exit 1)
	$(COMPOSE) exec api sh -lc "$(CMD)"

exec-ui:
	@test -n "$(CMD)" || (echo "Usage: make exec-ui CMD='...'" && exit 1)
	$(COMPOSE) exec ui sh -lc "$(CMD)"

exec-qdrant:
	@test -n "$(CMD)" || (echo "Usage: make exec-qdrant CMD='...'" && exit 1)
	$(COMPOSE) exec qdrant sh -lc "$(CMD)"

ingest:
	$(COMPOSE) run --rm ingest python ingest.py

reingest:
	$(COMPOSE) run --rm ingest python ingest.py --reset

ingest-shell:
	$(COMPOSE) run --rm ingest sh
