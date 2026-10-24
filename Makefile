.PHONY: help up up-deps down restart logs status db-shell db-seed db-reset redis-cli redis-flush test build run

# Load environment variables if .env exists
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

help: ## Show this help message
	@echo "AdPulse Engine - Available commands:"
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

up-deps: ## Start PostgreSQL and Redis containers in background
	docker compose up -d postgres redis

up: ## Start all containers (Postgres, Redis, and Go App) in background
	docker compose up --build -d

down: ## Stop all Docker containers
	docker compose down

restart: ## Restart Docker containers
	docker compose restart

logs: ## Stream logs from all containers
	docker compose logs -f

status: ## Show status of Docker containers
	docker compose ps

build: ## Build Go server binary locally
	go build -o bin/server ./cmd/server

run: ## Run Go server locally
	go run ./cmd/server

test: ## Run all unit and integration tests
	go test -v -count=1 ./...

db-shell: ## Open interactive PostgreSQL psql shell
	docker compose exec -it postgres psql -U $${POSTGRES_USER:-adserver} -d $${POSTGRES_DB:-adserver_db}

db-seed: ## Re-apply initial seed data to PostgreSQL
	docker compose exec -T postgres psql -U $${POSTGRES_USER:-adserver} -d $${POSTGRES_DB:-adserver_db} -f /docker-entrypoint-initdb.d/02_seed.sql

db-reset: ## Reset database volume and re-initialize schema & seed
	docker compose down -v
	docker compose up -d postgres redis

redis-cli: ## Open interactive Redis CLI
	docker compose exec -it redis redis-cli

redis-flush: ## Flush all Redis cache keys
	docker compose exec redis redis-cli flushall

bench: ## Run high-concurrency benchmark (5000 requests, 50 workers, mixed workload)
	go run ./cmd/bench -requests 5000 -concurrency 50 -scenario mixed

bench-ad: ## Run high-concurrency ad-serving latency benchmark
	go run ./cmd/bench -requests 5000 -concurrency 50 -scenario ad

bench-imp: ## Run high-concurrency impression ingestion benchmark
	go run ./cmd/bench -requests 5000 -concurrency 50 -scenario impression
