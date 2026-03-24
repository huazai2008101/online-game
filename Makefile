# Makefile for online-game platform
# Author: online-game team
# Description: Build, test, and development automation

# Project variables
PROJECT_NAME := online-game
GO_VERSION := 1.23
DOCKER_REGISTRY := ghcr.io
DOCKER_IMAGE := $(DOCKER_REGISTRY)/$(PROJECT_NAME)

# Git variables
GIT_COMMIT := $(shell git rev-parse --short HEAD)
GIT_TAG := $(shell git describe --tags --abbrev=0 --always 2>/dev/null)
GIT_DIRTY := $(shell test -n "$$(git diff --shortstat 2>/dev/null)" && echo "-dirty" || echo "")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION := $(GIT_TAG)$(GIT_DIRTY)

# Build variables
LDFLAGS := -X main.Version=$(VERSION) \
           -X main.GitCommit=$(GIT_COMMIT) \
           -X main.BuildTime=$(BUILD_TIME) \
           -s -w

# Go variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt
GOLINT := golangci-lint

# Directories
CMD_DIR := ./cmd
INTERNAL_DIR := ./internal
PKG_DIR := ./pkg
API_DIR := ./api
PROTO_DIR := ./proto
SCRIPTS_DIR := ./scripts
BUILD_DIR := ./build
DIST_DIR := ./dist
COVERAGE_DIR := ./coverage

# Services
SERVICES := gateway ws-gateway game-service user-service payment-service match-service

# Colors for output
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m
COLOR_RED := \033[31m

## help: Show this help message
.PHONY: help
help:
	@echo '$(COLOR_BOLD)$(PROJECT_NAME) Makefile$(COLOR_RESET)'
	@echo ''
	@echo 'Usage:'
	@echo '  make <target>'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## ===================================================================
## Development Commands
## ===================================================================

## dev: Run all services in development mode with hot reload
.PHONY: dev
dev:
	@echo "$(COLOR_GREEN)Starting development environment...$(COLOR_RESET)"
	@docker-compose -f docker-compose.dev.yml up -d
	@echo "$(COLOR_GREEN)Development environment started!$(COLOR_RESET)"
	@echo "Services:"
	@echo "  - API Gateway: http://localhost:8080"
	@echo "  - WebSocket Gateway: ws://localhost:8081"
	@echo "  - Prometheus: http://localhost:9090"
	@echo "  - Grafana: http://localhost:3000"

## dev-down: Stop development environment
.PHONY: dev-down
dev-down:
	@echo "$(COLOR_YELLOW)Stopping development environment...$(COLOR_RESET)"
	@docker-compose -f docker-compose.dev.yml down

## dev-logs: Show development logs
.PHONY: dev-logs
dev-logs:
	@docker-compose -f docker-compose.dev.yml logs -f

## dev-reset: Reset development environment (remove volumes)
.PHONY: dev-reset
dev-reset:
	@echo "$(COLOR_RED)Resetting development environment...$(COLOR_RESET)"
	@docker-compose -f docker-compose.dev.yml down -v
	@docker-compose -f docker-compose.dev.yml up -d

## ===================================================================
## Build Commands
## ===================================================================

## build: Build all services
.PHONY: build
build: clean proto-gen
	@echo "$(COLOR_GREEN)Building all services...$(COLOR_RESET)"
	@mkdir -p $(BUILD_DIR)
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$$service ./$(CMD_DIR)/$$service; \
	done
	@echo "$(COLOR_GREEN)Build complete!$(COLOR_RESET)"

## build-%: Build specific service (e.g., make build-gateway)
.PHONY: build-%
build-%: proto-gen
	@echo "$(COLOR_GREEN)Building $*...$(COLOR_RESET)"
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$* ./$(CMD_DIR)/$*

## build-linux: Build all services for Linux
.PHONY: build-linux
build-linux: proto-gen
	@echo "$(COLOR_GREEN)Building for Linux...$(COLOR_RESET)"
	@mkdir -p $(DIST_DIR)/linux
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/linux/$$service ./$(CMD_DIR)/$$service; \
	done
	@echo "$(COLOR_GREEN)Linux build complete!$(COLOR_RESET)"

## build-darwin: Build all services for macOS
.PHONY: build-darwin
build-darwin: proto-gen
	@echo "$(COLOR_GREEN)Building for macOS...$(COLOR_RESET)"
	@mkdir -p $(DIST_DIR)/darwin
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/darwin/$$service ./$(CMD_DIR)/$$service; \
	done
	@echo "$(COLOR_GREEN)macOS build complete!$(COLOR_RESET)"

## build-windows: Build all services for Windows
.PHONY: build-windows
build-windows: proto-gen
	@echo "$(COLOR_GREEN)Building for Windows...$(COLOR_RESET)"
	@mkdir -p $(DIST_DIR)/windows
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/windows/$$service.exe ./$(CMD_DIR)/$$service; \
	done
	@echo "$(COLOR_GREEN)Windows build complete!$(COLOR_RESET)"

## build-all: Build for all platforms
.PHONY: build-all
build-all: build-linux build-darwin build-windows

## ===================================================================
## Test Commands
## ===================================================================

## test: Run all tests
.PHONY: test
test:
	@echo "$(COLOR_GREEN)Running tests...$(COLOR_RESET)"
	$(GOTEST) -v -race -count=1 ./...

## test-unit: Run unit tests only
.PHONY: test-unit
test-unit:
	@echo "$(COLOR_GREEN)Running unit tests...$(COLOR_RESET)"
	$(GOTEST) -v -short -race ./...

## test-integration: Run integration tests only
.PHONY: test-integration
test-integration:
	@echo "$(COLOR_GREEN)Running integration tests...$(COLOR_RESET)"
	$(GOTEST) -v -race -tags=integration ./...

## test-cover: Run tests with coverage
.PHONY: test-cover
test-cover:
	@echo "$(COLOR_GREEN)Running tests with coverage...$(COLOR_RESET)"
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -v -race -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "$(COLOR_GREEN)Coverage report: $(COVERAGE_DIR)/coverage.html$(COLOR_RESET)"

## test-cover-func: Show function coverage
.PHONY: test-cover-func
test-cover-func:
	@echo "$(COLOR_GREEN)Function coverage:$(COLOR_RESET)"
	$(GOTEST) -v -coverprofile=/dev/null ./... | grep -E "^ok|coverage:"

## benchmark: Run benchmarks
.PHONY: benchmark
benchmark:
	@echo "$(COLOR_GREEN)Running benchmarks...$(COLOR_RESET)"
	$(GOTEST) -bench=. -benchmem -run=^$$ ./...

## ===================================================================
## Code Quality Commands
## ===================================================================

## lint: Run linters
.PHONY: lint
lint:
	@echo "$(COLOR_GREEN)Running linters...$(COLOR_RESET)"
	$(GOLINT) run --config .golangci.yml ./...

## lint-fix: Run linters with auto-fix
.PHONY: lint-fix
lint-fix:
	@echo "$(COLOR_GREEN)Running linters with auto-fix...$(COLOR_RESET)"
	$(GOLINT) run --config .golangci.yml --fix ./...

## fmt: Format code
.PHONY: fmt
fmt:
	@echo "$(COLOR_GREEN)Formatting code...$(COLOR_RESET)"
	$(GOFMT) -s -w .

## fmt-check: Check if code is formatted
.PHONY: fmt-check
fmt-check:
	@echo "$(COLOR_GREEN)Checking code formatting...$(COLOR_RESET)"
	@test -z "$$($(GOFMT) -s -l . | tee /dev/stderr)" || (echo "Code is not formatted. Run 'make fmt' to fix." && exit 1)

## imports: Format imports
.PHONY: imports
imports:
	@echo "$(COLOR_GREEN)Formatting imports...$(COLOR_RESET)"
	@go install golang.org/x/tools/cmd/goimports@latest
	@goimports -w -local $(PROJECT_NAME) .

## vet: Run go vet
.PHONY: vet
vet:
	@echo "$(COLOR_GREEN)Running go vet...$(COLOR_RESET)"
	$(GOCMD) vet ./...

## check: Run all checks (fmt, vet, lint, test)
.PHONY: check
check: fmt-check vet lint test
	@echo "$(COLOR_GREEN)All checks passed!$(COLOR_RESET)"

## ===================================================================
## Dependencies Commands
## ===================================================================

## deps: Download dependencies
.PHONY: deps
deps:
	@echo "$(COLOR_GREEN)Downloading dependencies...$(COLOR_RESET)"
	$(GOMOD) download
	$(GOMOD) verify

## deps-tidy: Tidy dependencies
.PHONY: deps-tidy
deps:
	@echo "$(COLOR_GREEN)Tidying dependencies...$(COLOR_RESET)"
	$(GOMOD) tidy

## deps-update: Update dependencies
.PHONY: deps-update
deps-update:
	@echo "$(COLOR_GREEN)Updating dependencies...$(COLOR_RESET)"
	$(GOGET) -u ./...
	$(GOMOD) tidy

## deps-check: Check for outdated dependencies
.PHONY: deps-check
deps-check:
	@echo "$(COLOR_GREEN)Checking for outdated dependencies...$(COLOR_RESET)"
	$(GOGET) -u ./...
	$(GOMOD) tidy

## ===================================================================
## Protocol Buffers Commands
## ===================================================================

## proto-gen: Generate protobuf files
.PHONY: proto-gen
proto-gen:
	@echo "$(COLOR_GREEN)Generating protobuf files...$(COLOR_RESET)"
	@if [ -d "$(PROTO_DIR)" ]; then \
		for dir in $(PROTO_DIR)/*/; do \
			if [ -f "$${dir}*.proto" ]; then \
				protoc \
					--proto_path=$(PROTO_DIR) \
					--go_out=$(PKG_DIR) \
					--go_opt=paths=source_relative \
					--go-grpc_out=$(PKG_DIR) \
					--go-grpc_opt=paths=source_relative \
					--grpc-gateway_out=$(PKG_DIR) \
					--grpc-gateway_opt=paths=source_relative \
					--openapiv2_out=$(API_DIR) \
					$${dir}*.proto; \
			fi; \
		done; \
	fi
	@echo "$(COLOR_GREEN)Protobuf generation complete!$(COLOR_RESET)"

## proto-lint: Lint protobuf files
.PHONY: proto-lint
proto-lint:
	@echo "$(COLOR_GREEN)Linting protobuf files...$(COLOR_RESET)"
	@if command -v buf >/dev/null 2>&1; then \
		buf lint; \
	else \
		echo "buf not installed. Install with: go install github.com/bufbuild/buf/cmd/buf@latest"; \
	fi

## proto-format: Format protobuf files
.PHONY: proto-format
proto-format:
	@echo "$(COLOR_GREEN)Formatting protobuf files...$(COLOR_RESET)"
	@if command -v buf >/dev/null 2>&1; then \
		buf format -w; \
	else \
		echo "buf not installed. Install with: go install github.com/bufbuild/buf/cmd/buf@latest"; \
	fi

## ===================================================================
## Docker Commands
## ===================================================================

## docker-build: Build Docker images for all services
.PHONY: docker-build
docker-build:
	@echo "$(COLOR_GREEN)Building Docker images...$(COLOR_RESET)"
	@for service in $(SERVICES); do \
		echo "Building $$service image..."; \
		docker build -t $(DOCKER_IMAGE)-$$service:$(VERSION) -f docker/$$service/Dockerfile .; \
	done
	@echo "$(COLOR_GREEN)Docker images built!$(COLOR_RESET)"

## docker-push: Push Docker images to registry
.PHONY: docker-push
docker-push:
	@echo "$(COLOR_GREEN)Pushing Docker images...$(COLOR_RESET)"
	@for service in $(SERVICES); do \
		echo "Pushing $$service image..."; \
		docker push $(DOCKER_IMAGE)-$$service:$(VERSION); \
	done
	@echo "$(COLOR_GREEN)Docker images pushed!$(COLOR_RESET)"

## docker-tag-latest: Tag images as latest
.PHONY: docker-tag-latest
docker-tag-latest:
	@for service in $(SERVICES); do \
		docker tag $(DOCKER_IMAGE)-$$service:$(VERSION) $(DOCKER_IMAGE)-$$service:latest; \
	done

## ===================================================================
## Database Commands
## ===================================================================

## db-up: Start database services
.PHONY: db-up
db-up:
	@echo "$(COLOR_GREEN)Starting database services...$(COLOR_RESET)"
	docker-compose -f docker-compose.db.yml up -d

## db-down: Stop database services
.PHONY: db-down
db-down:
	@echo "$(COLOR_YELLOW)Stopping database services...$(COLOR_RESET)"
	docker-compose -f docker-compose.db.yml down

## db-migrate: Run database migrations
.PHONY: db-migrate
db-migrate:
	@echo "$(COLOR_GREEN)Running database migrations...$(COLOR_RESET)"
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	migrate -path ./migrations -database "postgresql://localhost:5432/online_game?sslmode=disable" up

## db-migrate-create: Create a new migration (usage: make db-migrate-create NAME=add_users_table)
.PHONY: db-migrate-create
db-migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "Usage: make db-migrate-create NAME=migration_name"; \
		exit 1; \
	fi
	@migrate create -ext sql -dir ./migrations $(NAME)

## db-rollback: Rollback last migration
.PHONY: db-rollback
db-rollback:
	@echo "$(COLOR_YELLOW)Rolling back last migration...$(COLOR_RESET)"
	migrate -path ./migrations -database "postgresql://localhost:5432/online_game?sslmode=disable" down 1

## db-reset: Reset database (drop and recreate)
.PHONY: db-reset
db-reset:
	@echo "$(COLOR_RED)Resetting database...$(COLOR_RESET)"
	docker-compose -f docker-compose.db.yml down -v
	docker-compose -f docker-compose.db.yml up -d
	@sleep 3
	make db-migrate

## db-console: Open PostgreSQL console
.PHONY: db-console
db-console:
	docker exec -it online-game-postgres psql -U postgres -d online_game

## redis-cli: Open Redis CLI
.PHONY: redis-cli
redis-cli:
	docker exec -it online-game-redis redis-cli

## ===================================================================
## Clean Commands
## ===================================================================

## clean: Clean build artifacts
.PHONY: clean
clean:
	@echo "$(COLOR_YELLOW)Cleaning build artifacts...$(COLOR_RESET)"
	@rm -rf $(BUILD_DIR)
	@rm -rf $(DIST_DIR)
	@rm -rf $(COVERAGE_DIR)
	@echo "$(COLOR_GREEN)Clean complete!$(COLOR_RESET)"

## clean-all: Clean everything including dependencies
.PHONY: clean-all
clean-all: clean
	@echo "$(COLOR_YELLOW)Cleaning dependencies...$(COLOR_RESET)"
	@rm -rf vendor/
	@echo "$(COLOR_GREEN)Full clean complete!$(COLOR_RESET)"

## clean-docker: Clean Docker resources
.PHONY: clean-docker
clean-docker:
	@echo "$(COLOR_YELLOW)Cleaning Docker resources...$(COLOR_RESET)"
	@docker-compose -f docker-compose.dev.yml down -v
	@docker system prune -f
	@echo "$(COLOR_GREEN)Docker clean complete!$(COLOR_RESET)"

## ===================================================================
## Deployment Commands
## ===================================================================

## deploy-staging: Deploy to staging environment
.PHONY: deploy-staging
deploy-staging:
	@echo "$(COLOR_GREEN)Deploying to staging...$(COLOR_RESET)"
	@kubectl config use-context staging
	@helm upgrade --install $(PROJECT_NAME) ./helm/chart \
		--namespace staging \
		--set image.tag=$(VERSION) \
		--values ./helm/values-staging.yaml

## deploy-production: Deploy to production environment
.PHONY: deploy-production
deploy-production:
	@echo "$(COLOR_YELLOW)Deploying to production...$(COLOR_RESET)"
	@echo "Are you sure? This will affect production! [y/N]"
	@read -r response; \
	if [ "$$response" = "y" ] || [ "$$response" = "Y" ]; then \
		kubectl config use-context production; \
		helm upgrade --install $(PROJECT_NAME) ./helm/chart \
			--namespace production \
			--set image.tag=$(VERSION) \
			--values ./helm/values-production.yaml; \
	fi

## deploy-status: Check deployment status
.PHONY: deploy-status
deploy-status:
	@echo "$(COLOR_GREEN)Deployment status:$(COLOR_RESET)"
	@kubectl get pods -l app=$(PROJECT_NAME)
	@kubectl get services -l app=$(PROJECT_NAME)

## deploy-logs: Show deployment logs
.PHONY: deploy-logs
deploy-logs:
	@kubectl logs -l app=$(PROJECT_NAME) --all-containers=true -f --tail=100

## ===================================================================
## Monitoring Commands
## ===================================================================

## monitor-up: Start monitoring services
.PHONY: monitor-up
monitor-up:
	@echo "$(COLOR_GREEN)Starting monitoring services...$(COLOR_RESET)"
	docker-compose -f docker-compose.monitoring.yml up -d

## monitor-down: Stop monitoring services
.PHONY: monitor-down
monitor-down:
	@echo "$(COLOR_YELLOW)Stopping monitoring services...$(COLOR_RESET)"
	docker-compose -f docker-compose.monitoring.yml down

## monitor-logs: Show monitoring logs
.PHONY: monitor-logs
monitor-logs:
	docker-compose -f docker-compose.monitoring.yml logs -f

## ===================================================================
## Release Commands
## ===================================================================

## release: Create a new release (usage: make release VERSION=v1.0.0)
.PHONY: release
release:
	@if [ -z "$(VERSION)" ]; then \
		echo "Usage: make release VERSION=v1.0.0"; \
		exit 1; \
	fi
	@echo "$(COLOR_GREEN)Creating release $(VERSION)...$(COLOR_RESET)"
	@git tag -a $(VERSION) -m "Release $(VERSION)"
	@git push origin $(VERSION)
	@gh release create $(VERSION) --generate-notes

## release-snapshot: Create a snapshot release
.PHONY: release-snapshot
release-snapshot: build-all
	@echo "$(COLOR_GREEN)Creating snapshot release...$(COLOR_RESET)"
	@mkdir -p $(DIST_DIR)/snapshot
	@tar -czf $(DIST_DIR)/snapshot/$(PROJECT_NAME)-$(VERSION)-linux-amd64.tar.gz -C $(DIST_DIR)/linux .
	@tar -czf $(DIST_DIR)/snapshot/$(PROJECT_NAME)-$(VERSION)-darwin-amd64.tar.gz -C $(DIST_DIR)/darwin .
	@tar -czf $(DIST_DIR)/snapshot/$(PROJECT_NAME)-$(VERSION)-windows-amd64.tar.gz -C $(DIST_DIR)/windows .
	@echo "$(COLOR_GREEN)Snapshot release created in $(DIST_DIR)/snapshot$(COLOR_RESET)"

## ===================================================================
## Utility Commands
## ===================================================================

## install-tools: Install development tools
.PHONY: install-tools
install-tools:
	@echo "$(COLOR_GREEN)Installing development tools...$(COLOR_RESET)"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@go install github.com/bufbuild/buf/cmd/buf@latest
	@go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@latest
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "$(COLOR_GREEN)Tools installed!$(COLOR_RESET)"

## generate: Run all code generation
.PHONY: generate
generate: proto-gen
	@echo "$(COLOR_GREEN)Running go generate...$(COLOR_RESET)"
	$(GOCMD) generate ./...

## swagger: Generate Swagger documentation
.PHONY: swagger
swagger:
	@echo "$(COLOR_GREEN)Generating Swagger documentation...$(COLOR_RESET)"
	@swag init -g ./cmd/gateway/main.go -o ./api/swagger

## info: Show project information
.PHONY: info
info:
	@echo "$(COLOR_BOLD)Project Information:$(COLOR_RESET)"
	@echo "  Name:           $(PROJECT_NAME)"
	@echo "  Version:        $(VERSION)"
	@echo "  Git Commit:     $(GIT_COMMIT)"
	@echo "  Build Time:     $(BUILD_TIME)"
	@echo "  Go Version:     $(shell go version)"
	@echo "  Docker Registry: $(DOCKER_IMAGE)"
