# ---- AegisFlux Makefile ------------------------------------------------------
# Targets:
#   make up       -> build & start full stack (compose)
#   make down     -> stop & remove stack (including volumes)
#   make restart  -> down + up
#   make ps       -> list services
#   make logs     -> follow logs (use S=<svc> to filter)
#   make build    -> docker compose build
#   make rebuild  -> build with --no-cache
#   make seed     -> load demo data (waits for Neo4j)
#   make test     -> run project tests (script wrapper)
#   make health   -> quick health checks
#   make clean    -> remove dangling images/containers (safe)
#   make nuke     -> down + remove named volumes (destructive)

SHELL := /bin/bash
.ONESHELL:

# Paths
COMPOSE_FILE := infra/compose/docker-compose.yml
COMPOSE := docker compose -f $(COMPOSE_FILE)

# Env
ENV_FILE := .env
ENV_EXAMPLE := .env.example

# Named volumes (keep in sync with compose)
VOLUMES := neo4j-data ts-data

# Colors
C_INFO  := \033[1;36m
C_OK    := \033[1;32m
C_WARN  := \033[1;33m
C_ERR   := \033[1;31m
C_NONE  := \033[0m

.PHONY: up down restart ps logs build rebuild seed test health clean nuke env vault-up vault-bootstrap

env:
	@if [ ! -f $(ENV_FILE) ]; then \
	  echo -e "$(C_WARN)[env] $(ENV_FILE) not found. Copying from $(ENV_EXAMPLE)...$(C_NONE)"; \
	  cp $(ENV_EXAMPLE) $(ENV_FILE); \
	fi

up: env
	@echo -e "$(C_INFO)[up] building & starting containers$(C_NONE)"
	$(COMPOSE) up -d --build
	@echo -e "$(C_OK)[up] done$(C_NONE)"
	@echo "→ UI:       http://localhost:3000"
	@echo "→ API:      http://localhost:8080/docs"
	@echo "→ Neo4j:    http://localhost:7474"
	@echo "→ NATS:     nats://localhost:4222"
	@echo "→ Vault:    http://localhost:8200"

down:
	@echo -e "$(C_INFO)[down] stopping & removing containers$(C_NONE)"
	$(COMPOSE) down
	@echo -e "$(C_OK)[down] done$(C_NONE)"

restart:
	@$(MAKE) -s down
	@$(MAKE) -s up

ps:
	$(COMPOSE) ps

# Usage: make logs            # all services
#        make logs S=ui       # specific service
logs:
	@if [ -z "$(S)" ]; then \
	  $(COMPOSE) logs -f --tail=200; \
	else \
	  $(COMPOSE) logs -f --tail=200 $(S); \
	fi

build:
	$(COMPOSE) build

rebuild:
	$(COMPOSE) build --no-cache

# Wait for Neo4j, then run seed script
seed:
	@echo -e "$(C_INFO)[seed] waiting for Neo4j at http://localhost:7474 ...$(C_NONE)"
	@for i in {1..60}; do \
	  if curl -s -u "$${NEO4J_USER:-neo4j}:$${NEO4J_PASS:-password}" http://localhost:7474/ | grep -qi neo4j; then \
	    echo -e "$(C_OK)[seed] Neo4j is up$(C_NONE)"; break; \
	  fi; \
	  sleep 1; \
	done
	@echo -e "$(C_INFO)[seed] running scripts/seed$(C_NONE)"
	@bash scripts/seed

test:
	@echo -e "$(C_INFO)[test] running project tests$(C_NONE)"
	@bash scripts/test

health:
	@echo -e "$(C_INFO)[health] checking endpoints$(C_NONE)"
	@echo -n "Actions API  "; curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/health || true
	@echo -n "UI           "; curl -s -o /dev/null -w "%{http_code}\n" http://localhost:3000/ || true
	@echo -n "Neo4j        "; curl -s -o /dev/null -w "%{http_code}\n" http://localhost:7474/ || true

clean:
	@echo -e "$(C_INFO)[clean] removing dangling images/containers$(C_NONE)"
	-docker container prune -f
	-docker image prune -f
	@echo -e "$(C_OK)[clean] done$(C_NONE)"

nuke:
	@echo -e "$(C_ERR)[nuke] THIS WILL REMOVE NAMED VOLUMES: $(VOLUMES)$(C_NONE)"
	@read -p "Type 'NUKE' to continue: " ans; \
	if [ "$$ans" = "NUKE" ]; then \
	  $(COMPOSE) down -v; \
	  for v in $(VOLUMES); do docker volume rm -f $${PWD##*/}_$${v} 2>/dev/null || true; done; \
	  echo -e "$(C_OK)[nuke] volumes removed$(C_NONE)"; \
	else \
	  echo -e "$(C_WARN)[nuke] aborted$(C_NONE)"; \
	fi

# Vault targets
vault-up:
	@echo -e "$(C_INFO)[vault] starting Vault service$(C_NONE)"
	$(COMPOSE) up -d vault
	@echo -e "$(C_INFO)[vault] waiting for Vault to be ready...$(C_NONE)"
	@for i in {1..30}; do \
	  if curl -s http://localhost:8200/v1/sys/health >/dev/null 2>&1; then \
	    echo -e "$(C_OK)[vault] Vault is ready$(C_NONE)"; break; \
	  fi; \
	  sleep 1; \
	done
	@echo -e "$(C_OK)[vault] Vault available at http://localhost:8200$(C_NONE)"
	@echo "→ UI:       http://localhost:8200"
	@echo "→ Token:    root"

vault-bootstrap: vault-up
	@echo -e "$(C_INFO)[vault] bootstrapping Vault with initial secrets$(C_NONE)"
	@bash scripts/vault/dev-bootstrap.sh
	@echo -e "$(C_OK)[vault] bootstrap complete$(C_NONE)"

