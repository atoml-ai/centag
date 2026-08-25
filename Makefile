.PHONY: all build run clean test install help gen-init-sqlite frontend build-all copy-files harness-check package package-list

# Install-compatible layout (same as scripts/install.sh / scripts/lib/centag-layout.sh)
CENTAG_INSTALL_ROOT ?= $(HOME)/.centag
CENTAG_EDITION ?= personal
# Derive layout from INSTALL_ROOT only — ignore stale CENTAG_*_DIR exported in the shell.
CENTAG_BIN_DIR := $(CENTAG_INSTALL_ROOT)/bin
CENTAG_LIB_DIR := $(CENTAG_INSTALL_ROOT)/lib
CENTAG_VAR_DIR := $(CENTAG_INSTALL_ROOT)/var
BIN_DIR=$(CENTAG_LIB_DIR)/$(CENTAG_EDITION)
STATIC_DIR=$(BIN_DIR)/static
PATH_BIN_DIR=$(CENTAG_BIN_DIR)
PACKAGES_DIR=$(CENTAG_VAR_DIR)/packages
BINARY_NAME=centag-$(CENTAG_EDITION)
CMD_DIR=cmd
MAIN_FILE=$(CMD_DIR)/centag/main.go
# Product version for `centag version`: prefer version branch (feature/v0.2.7 → v0.2.7),
# then newest git tag; BUILD_TIME is only the build timestamp line.
VERSION ?= $(shell bash scripts/lib/centag-version.sh)
BUILD_TIME=$(shell date '+%Y-%m-%d %H:%M:%S')
LDFLAGS=-ldflags "-s -w -X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)'"
# 与 start.sh personal/team 全功能 tags 对齐；否则 backend_*/protocol_*/business_* 的 init 不会编译进来
# Core tags only; business_* plugins are optional via external go.mod.
BUILD_TAGS ?= protocol_openai,protocol_anthropic,protocol_gemini,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic,backend_gemini,backend_azure

# 第三方/渠道打包默认参数（根目录 packaging.env；可被命令行覆盖）
-include packaging.env
PACKAGE_ARCH ?= amd64
PACKAGE_MODE ?= native
PACKAGE_EDITION ?= minimal
PACKAGE_OUTPUT ?= $(PACKAGES_DIR)
TARGET ?= fnos
PACKAGE_ARGS ?=

# Refresh PATH wrapper + symlink under $(CENTAG_BIN_DIR)
link-install:
	@bash -c 'source scripts/lib/centag-layout.sh && \
		CENTAG_INSTALL_ROOT="$(CENTAG_INSTALL_ROOT)" CENTAG_BIN_DIR="$(CENTAG_BIN_DIR)" \
		centag_layout_use_edition "$(CENTAG_EDITION)" && \
		centag_install_edition_links "$(CENTAG_EDITION)"'

# Copy files to edition lib directory
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
	@echo "Building $(BINARY_NAME) → $(BIN_DIR)/"
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
	GOTOOLCHAIN=$(or $(GOTOOLCHAIN),auto) go build -tags '$(BUILD_TAGS)' $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@mkdir -p $(BIN_DIR)/static
	@$(MAKE) link-install
	@echo "Build complete: $(BIN_DIR)/$(BINARY_NAME)"

# Build frontend only (Vue 3 + Vite) → lib/<edition>/static
frontend:
	@echo "Building frontend → $(STATIC_DIR)..."
	@mkdir -p $(STATIC_DIR)
	@cd web && CENTAG_INSTALL_ROOT="$(CENTAG_INSTALL_ROOT)" CENTAG_EDITION="$(CENTAG_EDITION)" CENTAG_STATIC_DIR="$(abspath $(STATIC_DIR))" npm install && CENTAG_INSTALL_ROOT="$(CENTAG_INSTALL_ROOT)" CENTAG_EDITION="$(CENTAG_EDITION)" CENTAG_STATIC_DIR="$(abspath $(STATIC_DIR))" npm run build
	@echo "Frontend build complete: $(STATIC_DIR)"

# Build all (backend + frontend)
build-all: build frontend
	@echo "All builds complete!"

# Run via install-compatible wrapper (or binary beside layout)
run: build
	@echo "Running $(BINARY_NAME) via $(PATH_BIN_DIR)/centag ..."
	@if [ -x "$(PATH_BIN_DIR)/centag" ]; then \
		cd $(BIN_DIR) && "$(PATH_BIN_DIR)/centag"; \
	else \
		cd $(BIN_DIR) && ./$(BINARY_NAME); \
	fi

# Daemon
daemon: build
	@echo "Starting $(BINARY_NAME) with daemon..."
	LLM_PROXY_BINARY="$(abspath $(BIN_DIR)/$(BINARY_NAME))" ./scripts/tools/daemon.sh $(BIN_DIR)

# Daemon Debug
daemon-debug: build
	@echo "Starting $(BINARY_NAME) with daemon in debug mode..."
	DAEMON_DEBUG=true LLM_PROXY_BINARY="$(abspath $(BIN_DIR)/$(BINARY_NAME))" ./scripts/tools/daemon.sh $(BIN_DIR)

# Clean install-root build artifacts (keeps wrap/proxyctl runtime state)
clean:
	@echo "Cleaning $(CENTAG_INSTALL_ROOT) build artifacts..."
	rm -rf $(CENTAG_LIB_DIR)/personal $(CENTAG_LIB_DIR)/minimal
	rm -f $(PATH_BIN_DIR)/centag $(PATH_BIN_DIR)/centag-personal $(PATH_BIN_DIR)/centag-minimal $(PATH_BIN_DIR)/centag.cmd
	rm -rf $(CENTAG_VAR_DIR)/packages $(CENTAG_VAR_DIR)/release $(CENTAG_VAR_DIR)/cross
	rm -rf bin/server bin/packages bin/release
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

# Vet every nested plugin module in place. These are separate Go modules:
# the root ./... sweep never crosses module boundaries, and plain `make test`
# runs without feature tags so backend_*/protocol_* gated code is skipped.
# go vet type-checks _test.go too, which catches broken test-only imports
# across module boundaries (see C-1 regression: plugins/protocol/shared).
test-plugins:
	@echo "Vetting nested plugin modules (tags: $(BUILD_TAGS))..."
	@fail=0; \
	for d in $$(go list -m -f '{{.Dir}}' | grep '/plugins/'); do \
		if [ -z "$$(cd "$$d" && go list -tags '$(BUILD_TAGS)' ./... 2>/dev/null)" ]; then \
			continue; \
		fi; \
		echo "  vet $$d"; \
		(cd "$$d" && go vet -tags '$(BUILD_TAGS)' ./...) || fail=1; \
	done; \
	if [ $$fail -ne 0 ]; then echo "plugin module vet FAILED"; exit 1; fi; \
	echo "All plugin modules vet OK"

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

# 仓库布局/包列表卫生检查（CI 同名步骤）
harness-check:
	@bash scripts/check-harness-hygiene.sh

# Regenerate config/initdata/data/centag.db (checked into Git; run after schema or default policy changes)
gen-init-sqlite:
	go run ./cmd/migrate

# 第三方系统 / 渠道打包（fnOS 等）；默认参数见 packaging.env
package-list:
	@bash scripts/packaging/package.sh list

# FORM=cli|desktop OS=macos|linux|windows|fnos|docker ARCH=amd64|arm64|host|all
# 例: make package FORM=desktop OS=macos PACKAGE_ARGS='--skip-frontend'
#     make package FORM=cli OS=fnos ARCH=amd64
package:
	@if [ -n "$(FORM)" ] && [ -n "$(OS)" ]; then \
		bash scripts/packaging/package.sh $(FORM) $(OS) $(if $(ARCH),$(ARCH),) \
			$(if $(filter fnos,$(OS)),--mode $(PACKAGE_MODE) --edition $(PACKAGE_EDITION) --output $(PACKAGE_OUTPUT),) \
			$(PACKAGE_ARGS); \
	elif [ "$(TARGET)" = "fnos" ]; then \
		bash scripts/packaging/package.sh cli fnos \
			--mode $(PACKAGE_MODE) \
			--arch $(PACKAGE_ARCH) \
			--edition $(PACKAGE_EDITION) \
			--output $(PACKAGE_OUTPUT) \
			$(PACKAGE_ARGS); \
	else \
		bash scripts/packaging/package.sh $(TARGET) $(PACKAGE_ARGS); \
	fi

# Help
help:
	@echo "Available targets:"
	@echo "  make / make all   - Build backend + frontend (default)"
	@echo "  make build        - Build backend → \$$CENTAG_INSTALL_ROOT/lib/\$$CENTAG_EDITION/"
	@echo "  make frontend     - Build frontend → lib/<edition>/static"
	@echo "  make build-all    - Same as default (backend + frontend)"
	@echo "  make copy-files   - Copy configs and scripts to lib/<edition>/"
	@echo "  make gen-init-sqlite - Regenerate config/initdata/data/centag.db from code defaults"
	@echo "  make run          - Build backend and run via ~/.centag/bin/centag"
	@echo "  make daemon       - Build and run with daemon"
	@echo "  make daemon-debug - Build and run with daemon in debug mode"
	@echo "  make clean        - Clean ~/.centag lib/var build artifacts"
	@echo "  make test         - Run tests"
	@echo "  make install      - Install dependencies"
	@echo "  make fmt          - Format code"
	@echo "  make lint         - Run linter"
	@echo "  make harness-check - Repo layout / go list hygiene (scripts/check-harness-hygiene.sh)"
	@echo "  make package-list - List packaging targets (desktop/github/cli/fnos/...)"
	@echo "  make package      - Package for TARGET (default fnos); see ./start.sh package list"
	@echo ""
	@echo "Layout (override with CENTAG_INSTALL_ROOT / CENTAG_EDITION):"
	@echo "  INSTALL_ROOT=$(CENTAG_INSTALL_ROOT)"
	@echo "  binary=$(BIN_DIR)/$(BINARY_NAME)"
	@echo "  static=$(STATIC_DIR)"
	@echo "  packages=$(PACKAGES_DIR)"
	@echo ""
	@echo "Development:"
	@echo "  (cmd 入口: cmd/README.md；示例: archive/deprecated/examples/README.md)"
	@echo "  (文档索引: docs/README.md)"
	@echo "  (渠道打包: scripts/packaging/README.md / packaging.env)"
