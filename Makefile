.PHONY: all proto build run test clean docker-up docker-down docker-build

# Go parameters
BINARY_NAME=online-game
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags "-s -w"

# Service binaries
SERVICES=api-gateway user-service game-service admin-service

all: build

# ========== Proto ==========
proto:
	@echo "Generating proto files..."
	@for dir in proto/user proto/game; do \
		protoc --go_out=. --go_opt=paths=source_relative \
			--go-grpc_out=. --go-grpc_opt=paths=source_relative \
			$$dir/*.proto; \
	done

# ========== Build ==========
build: $(SERVICES)

api-gateway:
	$(GO) build $(LDFLAGS) -o bin/$@ ./cmd/$@

user-service:
	$(GO) build $(LDFLAGS) -o bin/$@ ./cmd/$@

game-service:
	$(GO) build $(LDFLAGS) -o bin/$@ ./cmd/$@

admin-service:
	$(GO) build $(LDFLAGS) -o bin/$@ ./cmd/$@

# ========== Run ==========
run-%:
	$(GO) run ./cmd/$*

# ========== Test ==========
test:
	$(GO) test ./pkg/... -v -count=1

test-actor:
	$(GO) test ./pkg/actor/... -v -count=1

test-engine:
	$(GO) test ./pkg/engine/... -v -count=1

# ========== Clean ==========
clean:
	rm -rf bin/ tmp/

# ========== Dependencies ==========
deps:
	$(GO) mod tidy
	$(GO) mod download

# ========== SDK ==========
sdk-server:
	cd sdks/server-sdk && npm run build

sdk-client:
	cd sdks/client-sdk && npm run build

# ========== Docker ==========
docker-build:
	@for svc in $(SERVICES); do \
		echo "Building $$svc..."; \
		docker build --build-arg SERVICE=$$svc -f deploy/docker/Dockerfile.service -t gameplatform/$$svc . || exit 1; \
	done

docker-up:
	docker compose -f deploy/docker-compose.yml up -d

docker-down:
	docker compose -f deploy/docker-compose.yml down

docker-logs:
	docker compose -f deploy/docker-compose.yml logs -f

docker-ps:
	docker compose -f deploy/docker-compose.yml ps

# ========== Local Development ==========
run-all:
	@echo "Starting all services..."
	@make run-user-service & \
	make run-game-service & \
	make run-admin-service & \
	make run-api-gateway & \
	wait
