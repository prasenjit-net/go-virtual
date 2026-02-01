.PHONY: all build build-go build-ui dev dev-server dev-ui clean test help install-deps

# Variables
BINARY_NAME=go-virtual
GO_CMD=go
UI_DIR=ui
BUILD_DIR=build
CMD_DIR=cmd/server

# Default target
all: build

# Install dependencies
install-deps:
	@echo "Installing Go dependencies..."
	$(GO_CMD) mod download
	$(GO_CMD) mod tidy
	@echo "Installing UI dependencies..."
	cd $(UI_DIR) && npm install

# Build everything
build: build-ui build-go
	@echo "Build complete!"

# Build UI
build-ui:
	@echo "Building UI..."
	cd $(UI_DIR) && npm run build
	@echo "UI build complete!"

# Build Go binary with embedded UI
build-go:
	@echo "Building Go binary..."
	@mkdir -p $(BUILD_DIR)
	$(GO_CMD) build -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "Go build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build Go binary without embedded UI (for development)
build-go-dev:
	@echo "Building Go binary (dev mode)..."
	@mkdir -p $(BUILD_DIR)
	$(GO_CMD) build -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "Go build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Run in development mode (separate terminals recommended)
dev: dev-server

# Run Go server in development mode
dev-server:
	@echo "Starting Go server in development mode..."
	$(GO_CMD) run ./$(CMD_DIR) -dev

# Run UI development server
dev-ui:
	@echo "Starting UI development server..."
	cd $(UI_DIR) && npm run dev

# Run both servers (requires concurrently or similar)
dev-all:
	@echo "Starting both servers..."
	@echo "Note: For best results, run 'make dev-server' and 'make dev-ui' in separate terminals"
	@command -v concurrently >/dev/null 2>&1 || (echo "Installing concurrently..." && npm install -g concurrently)
	concurrently "make dev-server" "make dev-ui"

# Run the production build
run:
	@echo "Running production build..."
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run with custom port
run-port:
	@echo "Running on port $(PORT)..."
	./$(BUILD_DIR)/$(BINARY_NAME) -port $(PORT)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -rf $(UI_DIR)/dist
	rm -rf $(UI_DIR)/node_modules
	@echo "Clean complete!"

# Clean only build artifacts (keep node_modules)
clean-build:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf $(UI_DIR)/dist
	@echo "Clean complete!"

# Run tests
test:
	@echo "Running Go tests..."
	$(GO_CMD) test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running Go tests with coverage..."
	$(GO_CMD) test -v -coverprofile=coverage.out ./...
	$(GO_CMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Lint Go code
lint:
	@echo "Linting Go code..."
	@command -v golangci-lint >/dev/null 2>&1 || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

# Lint UI code
lint-ui:
	@echo "Linting UI code..."
	cd $(UI_DIR) && npm run lint

# Format Go code
fmt:
	@echo "Formatting Go code..."
	$(GO_CMD) fmt ./...

# Generate (if needed)
generate:
	@echo "Running go generate..."
	$(GO_CMD) generate ./...

# Docker build
docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):latest .

# Docker run
docker-run:
	@echo "Running Docker container..."
	docker run -p 8080:8080 $(BINARY_NAME):latest

# Show help
help:
	@echo "Go-Virtual Makefile Commands:"
	@echo ""
	@echo "  make              - Build everything (UI + Go binary)"
	@echo "  make build        - Build everything (UI + Go binary)"
	@echo "  make build-ui     - Build UI only"
	@echo "  make build-go     - Build Go binary only"
	@echo "  make install-deps - Install all dependencies"
	@echo ""
	@echo "  make dev          - Run Go server in dev mode (serves UI from ./ui/dist)"
	@echo "  make dev-server   - Run Go server in dev mode"
	@echo "  make dev-ui       - Run Vite dev server for UI"
	@echo "  make dev-all      - Run both servers (needs concurrently)"
	@echo ""
	@echo "  make run          - Run the production binary"
	@echo "  make run-port PORT=3000 - Run on custom port"
	@echo ""
	@echo "  make test         - Run Go tests"
	@echo "  make test-coverage - Run tests with coverage"
	@echo "  make lint         - Lint Go code"
	@echo "  make lint-ui      - Lint UI code"
	@echo "  make fmt          - Format Go code"
	@echo ""
	@echo "  make clean        - Remove all build artifacts and node_modules"
	@echo "  make clean-build  - Remove build artifacts only"
	@echo ""
	@echo "  make docker-build - Build Docker image"
	@echo "  make docker-run   - Run Docker container"
	@echo ""
	@echo "  make help         - Show this help message"
