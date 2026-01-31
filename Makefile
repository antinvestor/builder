# =============================================================================
# Service Feature - Multi-Service Makefile
# =============================================================================
# Build and manage multiple independently deployable services.
#
# Services:
#   - gateway:  HTTP API entry point
#   - worker:   Main event processor
#   - reviewer: Security & architecture review
#   - executor: Sandbox test execution
# =============================================================================

.PHONY: help build up down logs clean test

# Variables
COMPOSE := docker compose
VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
SERVICES := gateway worker reviewer executor

# Colors
CYAN := \033[0;36m
GREEN := \033[0;32m
YELLOW := \033[1;33m
NC := \033[0m

.DEFAULT_GOAL := help

# =============================================================================
# Help
# =============================================================================

help: ## Show this help
	@echo ""
	@echo "$(CYAN)Service Feature - Multi-Service Platform$(NC)"
	@echo ""
	@echo "$(GREEN)Services:$(NC)"
	@echo "  gateway   - HTTP API entry point (stateless)"
	@echo "  worker    - Main event processor"
	@echo "  reviewer  - Security & architecture review"
	@echo "  executor  - Sandbox test execution"
	@echo ""
	@echo "$(GREEN)Quick Start:$(NC)"
	@echo "  make up          - Start all services"
	@echo "  make up-minimal  - Start gateway + worker + infra only"
	@echo "  make demo        - Run demo"
	@echo ""
	@echo "$(GREEN)Commands:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-18s$(NC) %s\n", $$1, $$2}'
	@echo ""

# =============================================================================
# Development
# =============================================================================

deps: ## Download Go dependencies
	go mod download
	go mod tidy

build: ## Build all services
	@for svc in $(SERVICES); do \
		echo "$(CYAN)Building $$svc...$(NC)"; \
		CGO_ENABLED=0 go build -ldflags="-w -s" \
			-o bin/$$svc ./apps/$$svc/cmd/main.go; \
	done
	@echo "$(GREEN)All services built in bin/$(NC)"

build-%: ## Build specific service (e.g., make build-gateway)
	@echo "$(CYAN)Building $*...$(NC)"
	CGO_ENABLED=0 go build -ldflags="-w -s" \
		-o bin/$* ./apps/$*/cmd/main.go
	@echo "$(GREEN)Built bin/$*$(NC)"

run-%: ## Run specific service locally (e.g., make run-gateway)
	go run ./apps/$*/cmd/main.go

fmt: ## Format code
	go fmt ./...

lint: ## Run linter
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run ./...

vet: ## Run go vet
	go vet ./...

# =============================================================================
# Docker - Individual Services
# =============================================================================

docker-build: ## Build all Docker images
	@for svc in $(SERVICES); do \
		echo "$(CYAN)Building Docker image for $$svc...$(NC)"; \
		docker build -f apps/$$svc/Dockerfile \
			--build-arg VERSION=$(VERSION) \
			-t feature-$$svc:$(VERSION) \
			-t feature-$$svc:latest .; \
	done
	@echo "$(GREEN)All images built$(NC)"

docker-build-%: ## Build specific Docker image (e.g., make docker-build-gateway)
	@echo "$(CYAN)Building Docker image for $*...$(NC)"
	docker build -f apps/$*/Dockerfile \
		--build-arg VERSION=$(VERSION) \
		-t feature-$*:$(VERSION) \
		-t feature-$*:latest .
	@echo "$(GREEN)Built feature-$*:$(VERSION)$(NC)"

# =============================================================================
# Docker Compose - Service Management
# =============================================================================

up: ## Start all services (core only, no observability)
	@echo "$(CYAN)Starting all services...$(NC)"
	$(COMPOSE) up -d gateway worker reviewer executor postgres nats
	@echo ""
	@echo "$(GREEN)Services started!$(NC)"
	@echo ""
	@echo "Gateway:  http://localhost:8080"
	@echo "NATS:     http://localhost:8222"
	@echo ""

up-minimal: ## Start minimal services (gateway + worker + infra)
	@echo "$(CYAN)Starting minimal services...$(NC)"
	$(COMPOSE) up -d gateway worker postgres nats
	@echo "$(GREEN)Minimal services started$(NC)"

up-full: ## Start all services including observability
	@echo "$(CYAN)Starting all services with observability...$(NC)"
	$(COMPOSE) --profile full up -d
	@echo ""
	@echo "$(GREEN)Full stack started!$(NC)"
	@echo ""
	@echo "Gateway:    http://localhost:8080"
	@echo "Grafana:    http://localhost:3000 (admin/admin)"
	@echo "Prometheus: http://localhost:9090"
	@echo "Jaeger:     http://localhost:16686"
	@echo ""

up-observability: ## Start observability stack only
	$(COMPOSE) --profile observability up -d

down: ## Stop all services
	$(COMPOSE) --profile full down

down-clean: ## Stop all services and remove volumes
	$(COMPOSE) --profile full down -v

restart: down up ## Restart all services

restart-%: ## Restart specific service (e.g., make restart-worker)
	$(COMPOSE) restart $*

# =============================================================================
# Scaling
# =============================================================================

scale-gateway: ## Scale gateway service (e.g., make scale-gateway N=3)
	$(COMPOSE) up -d --scale gateway=$(N) --no-recreate

scale-worker: ## Scale worker service (e.g., make scale-worker N=5)
	$(COMPOSE) up -d --scale worker=$(N) --no-recreate

scale-reviewer: ## Scale reviewer service (e.g., make scale-reviewer N=2)
	$(COMPOSE) up -d --scale reviewer=$(N) --no-recreate

scale-executor: ## Scale executor service (e.g., make scale-executor N=3)
	$(COMPOSE) up -d --scale executor=$(N) --no-recreate

# =============================================================================
# Logs
# =============================================================================

logs: ## View all service logs
	$(COMPOSE) logs -f gateway worker reviewer executor

logs-%: ## View specific service logs (e.g., make logs-worker)
	$(COMPOSE) logs -f $*

# =============================================================================
# Status
# =============================================================================

ps: ## Show running services
	$(COMPOSE) ps

health: ## Check all service health
	@echo "$(CYAN)Checking service health...$(NC)"
	@curl -sf http://localhost:8080/health && echo "Gateway:  $(GREEN)OK$(NC)" || echo "Gateway:  $(YELLOW)DOWN$(NC)"
	@echo ""

# =============================================================================
# Testing
# =============================================================================

test: ## Run all tests
	go test -v -race ./...

test-short: ## Run short tests
	go test -v -short ./...

test-coverage: ## Run tests with coverage
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report: coverage.html$(NC)"

test-%: ## Run tests for specific app (e.g., make test-worker)
	go test -v -race ./apps/$*/...

# =============================================================================
# Demo
# =============================================================================

demo: up ## Run demo
	@sleep 5
	./scripts/demo.sh

demo-quick: up-minimal ## Run quick demo
	@sleep 3
	./scripts/demo.sh --quick

demo-agents: ## Show agent demonstrations
	./scripts/demo.sh --agents

# =============================================================================
# Database
# =============================================================================

db-migrate: ## Run database migrations
	DATABASE_DO_MIGRATE=true go run ./apps/worker/cmd/main.go

db-shell: ## Open PostgreSQL shell
	docker exec -it feature-postgres psql -U feature -d feature

db-reset: ## Reset database
	docker exec -it feature-postgres psql -U feature -d feature -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
	docker exec -it feature-postgres psql -U feature -d feature -f /docker-entrypoint-initdb.d/init.sql

# =============================================================================
# NATS
# =============================================================================

nats-monitor: ## Monitor NATS subjects
	docker exec -it feature-nats nats sub "feature.>" --server nats://localhost:4222

nats-publish: ## Publish test message
	docker exec -it feature-nats nats pub feature.requests '{"test":true}' --server nats://localhost:4222

# =============================================================================
# Cleanup
# =============================================================================

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean -cache

clean-docker: ## Remove all Docker resources
	$(COMPOSE) --profile full down -v --rmi local
	docker system prune -f

clean-all: clean clean-docker ## Clean everything

# =============================================================================
# Utilities
# =============================================================================

version: ## Show version
	@echo "Version: $(VERSION)"
	@echo "Build time: $(BUILD_TIME)"

structure: ## Show project structure
	@echo "$(CYAN)Project Structure:$(NC)"
	@echo ""
	@echo "apps/"
	@echo "  gateway/   - HTTP API (stateless, scale horizontally)"
	@echo "  worker/    - Event processor (scale on queue depth)"
	@echo "  reviewer/  - Review agent (scale independently)"
	@echo "  executor/  - Sandbox execution (needs Docker socket)"
	@echo ""
	@echo "internal/"
	@echo "  events/    - Shared event types and payloads"
	@echo "  models/    - Shared data models"
	@echo "  utils/     - Shared utilities"
	@echo ""

env-example: ## Generate .env.example
	@cat > .env.example << 'EOF'
# Service Feature - Environment Configuration

# LLM API Keys
ANTHROPIC_API_KEY=your-anthropic-key
OPENAI_API_KEY=your-openai-key

# Git Authentication
# GIT_SSH_KEY_PATH=/path/to/ssh/key
# GIT_HTTPS_USERNAME=username
# GIT_HTTPS_PASSWORD=token

# Log level
LOG_LEVEL=info
EOF
	@echo "$(GREEN)Generated .env.example$(NC)"
