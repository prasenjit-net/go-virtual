# ──────────────────────────────────────────────────────────────────────────────
# Go-Virtual — Makefile
# ──────────────────────────────────────────────────────────────────────────────

BINARY     := go-virtual
BUILD_DIR  := build
CMD_DIR    := cmd/server
UI_DIR     := ui
GO         := go

# Docker Hub image name (override: make docker-push HUB_IMAGE=myorg/go-virtual)
HUB_IMAGE  ?= prasenjitnet/go-virtual

# Optional overrides (e.g.  make run PORT=9090)
PORT       ?=
CONFIG     ?=

# Version info baked into the binary
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LD_FLAGS   := -s -w \
	-X github.com/prasenjit/go-virtual/internal/version.Version=$(VERSION) \
	-X github.com/prasenjit/go-virtual/internal/version.Commit=$(COMMIT) \
	-X github.com/prasenjit/go-virtual/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: all build build-go build-ui \
        run run-init \
        dev dev-ui dev-all \
        test test-coverage \
        lint lint-ui fmt \
        install-deps clean clean-build \
        docker-build docker-build-local docker-push-local docker-run \
        compose-up compose-up-build compose-down compose-logs compose-ps \
        swarm-init swarm-deploy swarm-deploy-local swarm-status swarm-logs swarm-rm swarm-rm-volumes \
        help

# ── Default ───────────────────────────────────────────────────────────────────

all: build

# ── Build ─────────────────────────────────────────────────────────────────────

## build: Build UI then Go binary (full production build)
build: build-ui build-go

## build-ui: Build the React frontend only
build-ui:
	@echo "› Building UI…"
	cd $(UI_DIR) && npm run build
	@echo "✓ UI build complete"

## build-go: Build UI, then compile the Go binary with embedded assets
build-go: build-ui
	@echo "› Building Go binary ($(VERSION))…"
	@mkdir -p $(BUILD_DIR)
	$(GO) build -ldflags "$(LD_FLAGS)" -o $(BUILD_DIR)/$(BINARY) ./$(CMD_DIR)
	@echo "✓ Binary: $(BUILD_DIR)/$(BINARY)"

# ── Run ───────────────────────────────────────────────────────────────────────

## run: Build everything then start the server  (PORT=… to override port)
run: build
	@echo "› Starting $(BINARY) serve…"
	$(BUILD_DIR)/$(BINARY) serve \
		$(if $(PORT),--port $(PORT),) \
		$(if $(CONFIG),--config $(CONFIG),)

## run-init: Build everything then run 'init' to create default config + data dirs
run-init: build
	@echo "› Running $(BINARY) init…"
	$(BUILD_DIR)/$(BINARY) init \
		$(if $(CONFIG),--config $(CONFIG),)

# ── Development ───────────────────────────────────────────────────────────────

## dev: Run Go server in dev mode (hot-reload UI from ui/dist via filesystem)
dev:
	@echo "› Starting dev server (--dev flag)…"
	$(GO) run ./$(CMD_DIR) serve --dev \
		$(if $(PORT),--port $(PORT),) \
		$(if $(CONFIG),--config $(CONFIG),)

## dev-ui: Start the Vite dev server for the React frontend
dev-ui:
	@echo "› Starting Vite dev server…"
	cd $(UI_DIR) && npm run dev

## dev-all: Run Go server + Vite side-by-side (requires 'concurrently')
dev-all:
	@command -v concurrently >/dev/null 2>&1 \
		|| (echo "Installing concurrently…" && npm install -g concurrently)
	concurrently --names "server,ui" --prefix-colors "cyan,magenta" \
		"$(MAKE) dev" \
		"$(MAKE) dev-ui"

# ── Test & Quality ────────────────────────────────────────────────────────────

## test: Run all Go tests
test:
	@echo "› Running tests…"
	$(GO) test -tags unit -v -race ./...

## test-coverage: Run tests and produce coverage.html report
# Uses -tags unit to exclude MongoDB backend (requires live MongoDB) from
# coverage so that the file.go / memory.go unit tests drive the metric.
# Run with 'make test-coverage-integration' to include MongoDB (needs $MONGO_URI).
test-coverage:
	@echo "› Running tests with coverage…"
	$(GO) test -tags unit -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

## test-coverage-integration: Run tests including MongoDB integration tests
test-coverage-integration:
	@echo "› Running integration tests with coverage…"
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

## lint: Lint Go code (installs golangci-lint if missing)
lint:
	@command -v golangci-lint >/dev/null 2>&1 \
		|| (echo "Installing golangci-lint…" && $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

## lint-ui: Lint the React frontend
lint-ui:
	cd $(UI_DIR) && npm run lint

## fmt: Format Go source files
fmt:
	$(GO) fmt ./...

# ── Dependencies ──────────────────────────────────────────────────────────────

## install-deps: Install Go module deps and npm packages
install-deps:
	@echo "› Installing Go dependencies…"
	$(GO) mod download && $(GO) mod tidy
	@echo "› Installing UI dependencies…"
	cd $(UI_DIR) && npm install
	@echo "✓ Dependencies installed"

# ── Clean ─────────────────────────────────────────────────────────────────────

## clean: Remove build artifacts and node_modules
clean:
	rm -rf $(BUILD_DIR) $(UI_DIR)/dist $(UI_DIR)/node_modules
	@echo "✓ Cleaned"

## clean-build: Remove build artifacts only (keep node_modules)
clean-build:
	rm -rf $(BUILD_DIR) $(UI_DIR)/dist
	@echo "✓ Build artifacts cleaned"

# ── Docker ────────────────────────────────────────────────────────────────────

## docker-build-local: Build single-arch image for local use (no buildx needed)
docker-build-local:
	docker build -f Dockerfile.dev \
		-t $(BINARY):dev \
		-t $(BINARY):latest \
		-t $(HUB_IMAGE):latest \
		.

## docker-push-local: Build and push the local image to Docker Hub (single arch, for swarm testing)
docker-push-local: docker-build-local
	docker push $(HUB_IMAGE):latest
	@echo "✓ Pushed $(HUB_IMAGE):latest"

## docker-build: Build multi-arch image via buildx (requires buildx plugin)
docker-build:
	docker buildx build --platform linux/amd64,linux/arm64 \
		-t $(HUB_IMAGE):$(VERSION) -t $(HUB_IMAGE):latest --push .

## docker-run: Run the latest local image
docker-run:
	docker run --rm -p 8080:8080 -v "$(PWD)/data:/home/nonroot/data" $(BINARY):latest

# ── Docker Compose (local dev) ────────────────────────────────────────────────
# Uses Dockerfile.local — no buildx required, builds for native platform.

DEV_COMPOSE_FILE ?= docker-compose.yml
DOCKER_COMPOSE   ?= docker-compose

## compose-up: Start local dev stack (uses cached image if present)
compose-up:
	$(DOCKER_COMPOSE) -f $(DEV_COMPOSE_FILE) up -d

## compose-up-build: Build image from source and start local dev stack
compose-up-build:
	$(DOCKER_COMPOSE) -f $(DEV_COMPOSE_FILE) up -d --build

## compose-down: Stop and remove local dev containers (keeps ./data volume)
compose-down:
	$(DOCKER_COMPOSE) -f $(DEV_COMPOSE_FILE) down

## compose-logs: Tail logs from the local dev stack
compose-logs:
	$(DOCKER_COMPOSE) -f $(DEV_COMPOSE_FILE) logs -f

## compose-ps: Show status of local dev containers
compose-ps:
	$(DOCKER_COMPOSE) -f $(DEV_COMPOSE_FILE) ps

# ── Docker Swarm ──────────────────────────────────────────────────────────────

STACK_NAME ?= go-virtual
COMPOSE_FILE ?= swarm.yaml

## swarm-init: Initialise Docker Swarm on this node (skip if already active)
swarm-init:
	@docker info --format '{{.Swarm.LocalNodeState}}' | grep -q active \
		&& echo "✓ Swarm already active" \
		|| (docker swarm init && echo "✓ Swarm initialised")

## swarm-deploy: Deploy (or update) the stack using registry images (requires docker login)
swarm-deploy: swarm-init
	docker stack deploy --with-registry-auth -c $(COMPOSE_FILE) $(STACK_NAME)
	@echo "✓ Stack '$(STACK_NAME)' deployed"
	@echo "  Services:"
	@docker stack services $(STACK_NAME)

## swarm-deploy-local: Deploy using locally-built images (no registry push needed)
# Uses 'go-virtual:local' tag so Docker never tries to pull from a registry.
LOCAL_TAG := go-virtual:local
swarm-deploy-local: swarm-init
	docker build -f Dockerfile.dev -t $(LOCAL_TAG) .
	@# Substitute the Hub image name with the purely-local tag in a temp compose file
	@sed 's|image: $(HUB_IMAGE):.*|image: $(LOCAL_TAG)|g' $(COMPOSE_FILE) > /tmp/swarm-local.yaml
	docker stack deploy --resolve-image never -c /tmp/swarm-local.yaml $(STACK_NAME)
	@rm -f /tmp/swarm-local.yaml
	@echo "✓ Stack '$(STACK_NAME)' deployed (local image: $(LOCAL_TAG))"
	@echo "  Services:"
	@docker stack services $(STACK_NAME)

## swarm-status: Show stack services and their replica status
swarm-status:
	docker stack services $(STACK_NAME)

## swarm-logs: Tail logs for a service  (usage: make swarm-logs SVC=go-virtual)
SVC ?= go-virtual
swarm-logs:
	docker service logs --follow --tail 100 $(STACK_NAME)_$(SVC)

## swarm-rm: Remove the stack (containers + networks, keeps volumes)
swarm-rm:
	docker stack rm $(STACK_NAME)
	@echo "✓ Stack '$(STACK_NAME)' removed (volumes preserved)"

## swarm-rm-volumes: Remove the stack AND its named volumes
swarm-rm-volumes: swarm-rm
	@echo "Waiting for stack to settle before removing volumes…"
	@sleep 5
	docker volume rm $(STACK_NAME)_mongo-data $(STACK_NAME)_redis-data $(STACK_NAME)_redis-insight-data 2>/dev/null || true
	@echo "✓ Volumes removed"

# ── Help ──────────────────────────────────────────────────────────────────────

## help: Print this help message
help:
	@echo ""
	@echo "Usage: make <target> [VARIABLE=value …]"
	@echo ""
	@echo "Variables:"
	@echo "  PORT=8080      Override server port"
	@echo "  CONFIG=./c.yaml  Override config file path"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) \
		| sed 's/## //' \
		| awk -F': ' '{ printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }'
	@echo ""
