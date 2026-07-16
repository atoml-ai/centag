.PHONY: all build run clean test install help gen-init-sqlite frontend build-all copy-files harness-check

# Variables
BINARY_NAME=centag
BIN_DIR=bin/server
STATIC_DIR=bin/server/static
CMD_DIR=cmd
MAIN_FILE=$(CMD_DIR)/centag/main.go
VERSION=v$(shell date '+%Y%m%d-%H%M%S')
BUILD_TIME=$(shell date '+%Y-%m-%d %H:%M:%S')
LDFLAGS=-ldflags "-s -w -X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)'"
# 与 start.sh gateway/team 全功能 tags 对齐；否则 backend_*/protocol_*/business_* 的 init 不会编译进来
# Core tags only; business_* plugins are optional via external go.mod.
BUILD_TAGS ?= protocol_openai,protocol_anthropic,protocol_gemini,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic,backend_gemini,backend_azure

# Copy files to bin directory
copy-files:
	@echo "Copy static files to $(BIN_DIR)..."
	@mkdir -p $(BIN_DIR)/static
	@mkdir -p $(BIN_DIR)/storage
	@if [ -f "config/initdata/data/centag.db" ] && [ ! -f "$(BIN_DIR)/storage/centag.db" ]; then \
		ddb_line=$$( (grep -E '^[[:space:]]*(export[[:space:]]+)?LLM_PROXY_DB_DRIVER[[:space:]]*=' config/secrets/.env 2>/dev/null; grep -E '^[[:space:]]*(export[[:space:]]+)?LLM_PROXY_DB_DRIVER[[:space:]]*=' config/secrets/.env.middleware 2>/dev/null) | tail -1); \
		if echo "$$ddb_line" | grep -qiE '=[[:space:]]*(postgresql|postgres|pg)([[:space:]]|$$)'; then \
			echo "Skip SQLite seed (LLM_PROXY_DB_DRIVER is PostgreSQL)"; \
		else \
			cp "config/initdata/data/centag.db" "$(BIN_DIR)/storage/centag.db" && echo "Seeded $(BIN_DIR)/storage/centag.db from config/initdata (first time only)"; \
		fi; \
	fi
	@echo "Copy initdata to $(BIN_DIR)..."
	@if [ -d "config/initdata/scripts" ]; then \
		mkdir -p $(BIN_DIR)/scripts && cp -r config/initdata/scripts/* $(BIN_DIR)/scripts/ 2>/dev/null || true; \
	fi
	@if [ -d "config/initdata/update" ]; then \
		mkdir -p $(BIN_DIR)/update && cp -r config/initdata/update/* $(BIN_DIR)/update/ 2>/dev/null || true; \
	fi
	@if [ -d "config/initdata/rule" ]; then \
		mkdir -p $(BIN_DIR)/rule && cp -r config/initdata/rule/* $(BIN_DIR)/rule/ 2>/dev/null || true; \
	fi
	@echo "Copy scripts to $(BIN_DIR)..."
	@if [ -d "scripts" ]; then \
		mkdir -p $(BIN_DIR)/scripts; \
		cp scripts/*.sh $(BIN_DIR)/scripts/ 2>/dev/null || true; \
		chmod +x $(BIN_DIR)/scripts/*.sh 2>/dev/null || true; \
	fi
	@echo "Files copied to $(BIN_DIR)/"

# Default target: backend + frontend (matches refactor/project-structure)
all: build-all

# Build backend only
build:
	@echo "Building $(BINARY_NAME)..."
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@mkdir -p $(BIN_DIR)
	@mkdir -p $(BIN_DIR)/storage
	@if [ -f "config/initdata/data/centag.db" ] && [ ! -f "$(BIN_DIR)/storage/centag.db" ]; then \
		ddb_line=$$( (grep -E '^[[:space:]]*(export[[:space:]]+)?LLM_PROXY_DB_DRIVER[[:space:]]*=' config/secrets/.env 2>/dev/null; grep -E '^[[:space:]]*(export[[:space:]]+)?LLM_PROXY_DB_DRIVER[[:space:]]*=' config/secrets/.env.middleware 2>/dev/null) | tail -1); \
		if echo "$$ddb_line" | grep -qiE '=[[:space:]]*(postgresql|postgres|pg)([[:space:]]|$$)'; then \
			echo "Skip SQLite seed (LLM_PROXY_DB_DRIVER is PostgreSQL)"; \
		else \
			cp "config/initdata/data/centag.db" "$(BIN_DIR)/storage/centag.db" && echo "Seeded $(BIN_DIR)/storage/centag.db from config/initdata (first time only)"; \
		fi; \
	fi
	GOTOOLCHAIN=auto go build -tags '$(BUILD_TAGS)' $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "Copy static files to $(BIN_DIR)..."
	@mkdir -p $(BIN_DIR)/static
	@echo "Build complete: $(BIN_DIR)/"

# Build frontend only (Vue 3 + Vite)
frontend:
	@echo "Building frontend..."
	@cd web && npm install && npm run build
	@echo "Frontend build complete!"

# Build all (backend + frontend)
build-all: build frontend
	@echo "All builds complete!"

# Run
run: build
	@echo "Running $(BINARY_NAME) from $(BIN_DIR)..."
	cd $(BIN_DIR) && ./$(BINARY_NAME)

# Daemon
daemon: build
	@echo "Starting $(BINARY_NAME) with daemon..."
	./scripts/tools/daemon.sh $(BIN_DIR)

# Daemon Debug
daemon-debug: build
	@echo "Starting $(BINARY_NAME) with daemon in debug mode..."
	DAEMON_DEBUG=true ./scripts/tools/daemon.sh $(BIN_DIR)

# Clean
clean:
	@echo "Cleaning..."
	rm -rf bin/server bin/desktop bin/packages
	@echo "Clean complete"

# Test
test:
	@echo "Running tests..."
	@bash scripts/ci-go-packages.sh | xargs go test -count=1 -v

# Test with race detection
test-race:
	@echo "Running tests with race detector..."
	@bash scripts/ci-go-packages.sh | xargs go test -race -count=1
	@echo "Race detection passed"

# Install dependencies
install:
	@echo "Installing dependencies..."
	go mod tidy
	@echo "Dependencies installed"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Code formatted"

# Lint
lint:
	@echo "Running linter..."
	golangci-lint run
	@echo "Lint complete"

# Harness 文档/包列表卫生检查（CI 同名步骤）
harness-check:
	@bash scripts/check-harness-hygiene.sh

# Regenerate config/initdata/data/centag.db (checked into Git; run after schema or default policy changes)
gen-init-sqlite:
	go run ./cmd/migrate

# Help
help:
	@echo "Available targets:"
	@echo "  make / make all   - Build backend + frontend (default)"
	@echo "  make build        - Build backend binary only"
	@echo "  make frontend     - Build frontend only"
	@echo "  make build-all    - Same as default (backend + frontend)"
	@echo "  make copy-files   - Copy configs and scripts to bin/server/"
	@echo "  make gen-init-sqlite - Regenerate config/initdata/data/centag.db from code defaults"
	@echo "  make run          - Build backend and run from ./bin/server"
	@echo "  make daemon       - Build and run with daemon"
	@echo "  make daemon-debug - Build and run with daemon in debug mode"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make test         - Run tests"
	@echo "  make install      - Install dependencies"
	@echo "  make fmt          - Format code"
	@echo "  make lint         - Run linter"
	@echo "  make harness-check - Harness doc / go list hygiene (scripts/check-harness-hygiene.sh)"
	@echo ""
	@echo "Development:"
	@echo "  (cmd 入口: cmd/README.md；示例: archive/deprecated/examples/README.md)"
	@echo "  (文档索引: docs/README.md)"