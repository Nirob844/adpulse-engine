.PHONY: help up down restart logs status db-shell db-seed db-reset redis-cli redis-flush

# Load environment variables if .env exists
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

help: ## Show this help message
	@echo "Available commands:"
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

up: ## Start PostgreSQL and Redis containers in background
	docker compose up -d

down: ## Stop all Docker containers
	docker compose down

restart: ## Restart Docker containers
	docker compose restart

logs: ## Stream logs from all containers
	docker compose logs -f

status: ## Show status of Docker containers
	docker compose ps

db-shell: ## Open interactive PostgreSQL psql shell
	docker compose exec -it postgres psql -U $${POSTGRES_USER:-adserver} -d $${POSTGRES_DB:-adserver_db}

db-seed: ## Re-apply initial seed data to PostgreSQL
	docker compose exec -T postgres psql -U $${POSTGRES_USER:-adserver} -d $${POSTGRES_DB:-adserver_db} -f /docker-entrypoint-initdb.d/02_seed.sql

db-reset: ## Reset database volume and re-initialize schema & seed
	docker compose down -v
	docker compose up -d

redis-cli: ## Open interactive Redis CLI
	docker compose exec -it redis redis-cli

redis-flush: ## Flush all Redis cache keys
	docker compose exec redis redis-cli flushall
