.DEFAULT_GOAL := help

# ──────────────────────────────────────────────
# Docker Compose – lifecycle
# ──────────────────────────────────────────────

.PHONY: up down up-all down-all build rebuild logs logs-all

up: ## Start app + apprise (default profile)
	docker compose up -d

down: ## Stop default profile
	docker compose down

up-all: ## Start everything including monitoring stack
	docker compose --profile monitoring up -d

down-all: ## Stop everything including monitoring stack
	docker compose --profile monitoring down

build: ## Build Docker image
	docker compose build

rebuild: ## Rebuild image and restart containers
	docker compose up -d --build

logs: ## Tail pingpong container logs
	docker compose logs -f pingpong

logs-all: ## Tail all container logs
	docker compose logs -f

# ──────────────────────────────────────────────
# Go – build & test
# ──────────────────────────────────────────────

.PHONY: go-build test test-all vet tidy lint check

go-build: ## Build the binary locally
	go build ./cmd/pingpong/

test: ## Run tests (skip integration / long-running)
	go test -short ./...

test-all: ## Run all tests (needs CAP_NET_RAW for ping)
	go test ./...

vet: ## Static analysis
	go vet ./...

tidy: ## Tidy go.mod / go.sum
	go mod tidy

lint: ## Run golangci-lint (if installed)
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed – skipping"; exit 0; }
	golangci-lint run

check: vet test ## Pre-commit quality gate (vet + test + tidy check)
	@echo "Checking go.mod/go.sum are tidy…"
	go mod tidy
	@git diff --exit-code go.mod go.sum || { echo "go.mod/go.sum not tidy"; exit 1; }

# ──────────────────────────────────────────────
# Setup & cleanup
# ──────────────────────────────────────────────

.PHONY: env-setup clean

env-setup: ## Copy .env.example → .env (if .env is missing)
	@test -f .env && echo ".env already exists – skipping" || (cp .env.example .env && echo "Created .env from .env.example")

clean: ## Remove binary and Docker volumes
	rm -f pingpong
	docker compose --profile monitoring down -v

# ──────────────────────────────────────────────
# Help
# ──────────────────────────────────────────────

.PHONY: help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
