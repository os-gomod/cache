# =============================================================================
# Go Cache Library — github.com/os-gomod/go-cache
# Go version: 1.61.1
# =============================================================================

# -----------------------------------------------------------------------------
# Project Configuration
# -----------------------------------------------------------------------------
GO           := go
MODULE_PATH  := github.com/os-gomod/go-cache
PROJECT_NAME := cache
BINARY_NAME  := cache-example

# -----------------------------------------------------------------------------
# Directories
# -----------------------------------------------------------------------------
BIN_DIR      := bin
DIST_DIR     := $(BIN_DIR)/dist
COVERAGE_DIR := .coverage

# -----------------------------------------------------------------------------
# Quality gates
# -----------------------------------------------------------------------------
COVERAGE_THRESHOLD := 30

# -----------------------------------------------------------------------------
# Benchmark tuning
# -----------------------------------------------------------------------------
BENCH_TIME  := 1s
BENCH_COUNT := 5

# -----------------------------------------------------------------------------
# Linter configuration
# -----------------------------------------------------------------------------
LINT_CONFIG := .golangci.yml

# =============================================================================
# PHONY
# =============================================================================
.PHONY: help \
        deps deps-update deps-tidy \
        fmt vet lint lint-fix \
        staticcheck deadcode dupl \
        test test-race test-v test-verbose test-integration \
        coverage coverage-html coverage-ci coverage-threshold \
        benchmark benchmark-cpu benchmark-mem \
        build build-all cross-build build-example \
        generate \
        dev-tools install-tools \
        validate clean \
        docker-up docker-up-build docker-down docker-down-volumes \
        docker-logs docker-logs-redis docker-logs-commander docker-ps \
        docker-restart docker-redis-cli docker-stats docker-memory \
        docker-keys docker-flush docker-backup docker-monitor docker-test \
        docker-clean docker-prune

# =============================================================================
# HELP
# =============================================================================
help:
	@printf "\n"
	@printf "╔══════════════════════════════════════════════════════════════════╗\n"
	@printf "║      github.com/os-gomod/go-cache  —  Go 1.26.1 Cache Library    ║\n"
	@printf "╚══════════════════════════════════════════════════════════════════╝\n"
	@printf "\n"
	@printf "📦 Dependencies\n"
	@printf "    deps                    - Download + tidy dependencies\n"
	@printf "    deps-update             - go get -u + tidy\n"
	@printf "    deps-tidy               - go mod tidy\n"
	@printf "\n"
	@printf "🎨 Code Quality\n"
	@printf "    fmt                     - goimports + go fmt\n"
	@printf "    vet                     - go vet\n"
	@printf "    lint                    - golangci-lint\n"
	@printf "    lint-fix                - golangci-lint --fix\n"
	@printf "    staticcheck             - staticcheck\n"
	@printf "    deadcode                - deadcode\n"
	@printf "    dupl                    - gocyclo\n"
	@printf "\n"
	@printf "🧪 Testing\n"
	@printf "    test                    - Race-enabled tests with coverage\n"
	@printf "    test-race               - go test -race ./...\n"
	@printf "    test-v                  - Verbose tests, no race\n"
	@printf "    test-verbose            - Alias for test-v\n"
	@printf "    test-integration        - Build-tag 'integration' tests\n"
	@printf "\n"
	@printf "📊 Coverage\n"
	@printf "    coverage                - Coverage + threshold gate\n"
	@printf "    coverage-html           - HTML report\n"
	@printf "    coverage-ci             - CI report — no threshold enforcement\n"
	@printf "    coverage-threshold      - Check coverage threshold\n"
	@printf "\n"
	@printf "⚡ Benchmarks\n"
	@printf "    benchmark               - All benchmarks with -benchmem\n"
	@printf "    benchmark-cpu           - CPU profile → .coverage/prof/\n"
	@printf "    benchmark-mem           - Heap profile → .coverage/prof/\n"
	@printf "\n"
	@printf "🏗  Build\n"
	@printf "    build                   - Build all packages (compile-check)\n"
	@printf "    build-all               - Alias for build\n"
	@printf "    build-example           - Build example binary → bin/cache-example\n"
	@printf "    cross-build             - linux/darwin/windows × amd64/arm64\n"
	@printf "    generate                - go generate ./...\n"
	@printf "\n"
	@printf "🐳 Docker (Redis)\n"
	@printf "    docker-up               - Start Redis + Redis Commander\n"
	@printf "    docker-up-build         - Build and start services\n"
	@printf "    docker-down             - Stop services\n"
	@printf "    docker-down-volumes     - Stop services and remove volumes\n"
	@printf "    docker-logs             - Show all logs\n"
	@printf "    docker-logs-redis       - Show Redis logs\n"
	@printf "    docker-logs-commander   - Show Redis Commander logs\n"
	@printf "    docker-ps               - List services\n"
	@printf "    docker-restart          - Restart all services\n"
	@printf "    docker-redis-cli        - Open Redis CLI\n"
	@printf "    docker-stats            - Show Redis statistics\n"
	@printf "    docker-memory           - Show Redis memory usage\n"
	@printf "    docker-keys             - List Redis keys\n"
	@printf "    docker-flush            - Flush Redis database\n"
	@printf "    docker-backup           - Create Redis backup\n"
	@printf "    docker-monitor          - Monitor Redis commands\n"
	@printf "    docker-test             - Test Redis connection\n"
	@printf "    docker-clean            - Clean all Docker resources\n"
	@printf "    docker-prune            - Full Docker system prune\n"
	@printf "\n"
	@printf "🔧 Tools\n"
	@printf "    dev-tools               - Install core dev tools\n"
	@printf "    install-tools           - dev-tools + optional extras\n"
	@printf "    validate                - Full quality gate\n"
	@printf "\n"
	@printf "🧹 Cleanup\n"
	@printf "    clean                   - Remove bin/ .coverage/ dist/ prof files\n"
	@printf "\n"

# =============================================================================
# DEPENDENCIES
# =============================================================================

deps:
	@echo "📦 Downloading + tidying dependencies..."
	@$(GO) mod download
	@$(GO) mod tidy -v
	@echo "    dependencies ready. ✅"

deps-update:
	@echo "🔄 Updating dependencies..."
	@$(GO) get -u ./...
	@$(GO) mod tidy -v
	@echo "    dependencies updated. ✅"

deps-tidy:
	@echo "🧹 Tidying go.mod..."
	@$(GO) mod tidy -v
	@echo "    go.mod tidied. ✅"

# =============================================================================
# CODE QUALITY
# =============================================================================

fmt:
	@echo "🎨 Formatting code..."
	@$(GO) fmt ./...
	@command -v gofumpt >/dev/null 2>&1 && gofumpt -w . || true
	@command -v goimports >/dev/null 2>&1 && goimports -w . || true
	@command -v golines >/dev/null 2>&1 && golines -w . || true
	@echo "    formatting complete. ✅"

vet:
	@echo "🔬 Running go vet..."
	@$(GO) vet ./...
	@echo "    go vet passed. ✅"

lint:
	@echo "🔍 Linting..."
	@if [ -f "$(LINT_CONFIG)" ]; then \
		$(GO_LINT) run -c $(LINT_CONFIG) ./...; \
	else \
		$(GO_LINT) run ./...; \
	fi
	@echo "    linting passed. ✅"

lint-fix:
	@echo "🔧 Linting with --fix..."
	@if [ -f "$(LINT_CONFIG)" ]; then \
		$(GO_LINT) run -c $(LINT_CONFIG) --fix ./...; \
	else \
		$(GO_LINT) run --fix ./...; \
	fi
	@echo "    linting fixes applied. ✅"

staticcheck:
	@echo "🧠 Running staticcheck..."
	@staticcheck ./...
	@echo "    staticcheck passed. ✅"

deadcode:
	@echo "☠️ Checking dead code..."
	@deadcode ./...
	@echo "    deadcode check passed. ✅"

dupl:
	@echo "🔁 Checking duplication..."
	@dupl . 2>&1 | grep -v "dupl: found" || true
	@echo "    duplication check complete. ✅"

check: fmt vet lint staticcheck deadcode dupl
	@echo "✅ All checks complete"

# =============================================================================
# TESTING
# =============================================================================

test:
	@echo "🧪 Running tests with race detector..."
	@mkdir -p $(COVERAGE_DIR)
	@$(GO) test -race -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	@echo "    tests passed. ✅"

test-race:
	@echo "🏃 Running tests with race detector..."
	@$(GO) test -race ./...
	@echo "    tests passed. ✅"

test-v: test-verbose
test-verbose:
	@echo "🔊 Running tests (verbose)..."
	@$(GO) test -v ./...
	@echo "    tests passed. ✅"

test-integration:
	@echo "🔗 Running integration tests..."
	@$(GO) test -tags=integration ./...
	@echo "    integration tests passed. ✅"

# =============================================================================
# COVERAGE
# =============================================================================

coverage:
	@echo "📊 Running coverage..."
	@mkdir -p $(COVERAGE_DIR)
	@$(GO) test -race -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	@$(MAKE) --no-print-directory coverage-threshold
	@echo "    coverage complete. ✅"

coverage-html:
	@echo "🌐 Generating HTML coverage report..."
	@mkdir -p $(COVERAGE_DIR)
	@if [ ! -f "$(COVERAGE_DIR)/coverage.out" ]; then \
		$(GO) test -coverprofile=$(COVERAGE_DIR)/coverage.out ./...; \
	fi
	@$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "    report: $(COVERAGE_DIR)/coverage.html"

coverage-ci:
	@echo "🔍 CI coverage (no gate)..."
	@mkdir -p $(COVERAGE_DIR)
	@if [ ! -f "$(COVERAGE_DIR)/coverage.out" ]; then \
		$(GO) test -coverprofile=$(COVERAGE_DIR)/coverage.out ./...; \
	fi
	@pct=$$($(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out | grep ^total | awk '{print $$3}' | tr -d '%'); \
	printf "    Total coverage: %s%%\n" "$$pct"

coverage-threshold:
	@pct=$$($(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out 2>/dev/null | grep ^total | awk '{print $$3}' | tr -d '%'); \
	if [ -z "$$pct" ]; then \
		echo "    No coverage data found. Run 'make test' first."; \
		exit 1; \
	fi; \
	printf "    Coverage: %s%% (threshold: %s%%)\n" "$$pct" "$(COVERAGE_THRESHOLD)"; \
	if [ $$(echo "$$pct < $(COVERAGE_THRESHOLD)" | bc -l) -eq 1 ]; then \
		echo "❌ Below threshold"; \
		exit 1; \
	else \
		echo "✅ Threshold met"; \
	fi

# =============================================================================
# BENCHMARKS
# =============================================================================

benchmark:
	@echo "⚡ Running benchmarks..."
	@mkdir -p $(COVERAGE_DIR)
	@$(GO) test -bench=. -benchmem -benchtime=$(BENCH_TIME) -count=$(BENCH_COUNT) ./... | tee $(COVERAGE_DIR)/benchmark.txt
	@echo "    benchmarks complete. ✅"

benchmark-cpu:
	@echo "💻 CPU profiling benchmarks..."
	@mkdir -p $(COVERAGE_DIR)/prof
	@$(GO) test -bench=. -benchmem -benchtime=$(BENCH_TIME) \
		-cpuprofile=$(COVERAGE_DIR)/prof/cpu.prof ./...
	@echo "    CPU profile: $(COVERAGE_DIR)/prof/cpu.prof"

benchmark-mem:
	@echo "🧠 Memory profiling benchmarks..."
	@mkdir -p $(COVERAGE_DIR)/prof
	@$(GO) test -bench=. -benchmem -benchtime=$(BENCH_TIME) \
		-memprofile=$(COVERAGE_DIR)/prof/mem.prof ./...
	@echo "    Memory profile: $(COVERAGE_DIR)/prof/mem.prof"

# =============================================================================
# BUILD
# =============================================================================

build:
	@echo "🔨 Compile-checking all packages..."
	@$(GO) build ./...
	@echo "    build successful. ✅"

build-all: build

build-example:
	@echo "🔨 Building example binary..."
	@mkdir -p $(BIN_DIR)
	@$(GO) build -o $(BIN_DIR)/$(BINARY_NAME) ./example/...
	@echo "    binary: $(BIN_DIR)/$(BINARY_NAME)"

cross-build:
	@echo "🌍 Cross-compiling: linux/darwin/windows × amd64/arm64..."
	@mkdir -p $(DIST_DIR)
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
			out="$(DIST_DIR)/$(BINARY_NAME)-$$os-$$arch$$ext"; \
			printf "    %-40s" "$$os/$$arch"; \
			GOOS=$$os GOARCH=$$arch $(GO) build -o $$out ./example/... \
				&& echo "✅" || echo "❌"; \
		done; \
	done
	@echo "    cross-build complete → $(DIST_DIR)/"

generate:
	@echo "⚙️ Running go generate..."
	@$(GO) generate ./...
	@echo "    generation complete. ✅"

# =============================================================================
# DOCKER (Redis)
# =============================================================================

DOCKER_COMPOSE := docker compose
DOCKER         := docker

docker-up:
	@echo "🚀 Starting Docker Compose services..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml up -d
	@echo "✅ Services started"

docker-up-build:
	@echo "🔨 Building and starting Docker Compose services..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml up -d --build
	@echo "✅ Services built and started"

docker-down:
	@echo "🛑 Stopping Docker Compose services..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml down
	@echo "✅ Services stopped"

docker-down-volumes:
	@echo "🗑️ Stopping Docker Compose services and removing volumes..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml down -v
	@echo "✅ Services stopped and volumes removed"

docker-logs:
	@echo "📋 Showing Docker Compose logs..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml logs -f

docker-logs-redis:
	@echo "🔴 Showing Redis logs..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml logs -f redis

docker-logs-commander:
	@echo "📊 Showing Redis Commander logs..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml logs -f redis-commander

docker-ps:
	@echo "📊 Listing Docker Compose services..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml ps

docker-restart:
	@echo "🔄 Restarting Docker Compose services..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml restart
	@echo "✅ Services restarted"

docker-redis-cli:
	@echo "🛠️ Opening Redis CLI..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml exec redis redis-cli

docker-stats:
	@echo "📈 Showing Redis statistics..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml exec redis redis-cli info stats

docker-memory:
	@echo "🧠 Showing Redis memory usage..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml exec redis redis-cli info memory

docker-keys:
	@echo "🔑 Listing Redis keys..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml exec redis redis-cli --scan

docker-flush:
	@echo "🧹 Flushing Redis database..."
	@read -p "Are you sure you want to flush Redis? [y/N]: " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		$(DOCKER_COMPOSE) -f docker-compose.yml exec redis redis-cli flushall; \
		echo "✅ Redis flushed"; \
	else \
		echo "❌ Flush cancelled"; \
	fi

docker-backup:
	@echo "💾 Creating Redis backup..."
	@timestamp=$$(date +%Y%m%d_%H%M%S); \
	backup_dir="backups"; \
	mkdir -p $$backup_dir; \
	$(DOCKER_COMPOSE) -f docker-compose.yml exec -T redis redis-cli --rdb - > $$backup_dir/redis_backup_$$timestamp.rdb; \
	echo "✅ Backup saved to $$backup_dir/redis_backup_$$timestamp.rdb"

docker-monitor:
	@echo "👀 Monitoring Redis commands in real-time..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml exec redis redis-cli monitor

docker-test:
	@echo "🧪 Testing Redis connection..."
	@if $(DOCKER_COMPOSE) -f docker-compose.yml exec redis redis-cli ping | grep -q "PONG"; then \
		echo "✅ Redis is responding correctly"; \
	else \
		echo "❌ Redis is not responding"; \
		exit 1; \
	fi

docker-clean:
	@echo "🧹 Cleaning Docker Compose resources..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml down -v --rmi local
	@echo "✅ Docker Compose resources cleaned"

docker-prune:
	@echo "🧹 Cleaning Docker system..."
	@$(DOCKER_COMPOSE) -f docker-compose.yml down -v --rmi local --volumes
	@$(DOCKER) system prune -af --volumes
	@echo "✅ Docker system cleaned"

# =============================================================================
# TOOLS
# =============================================================================

GO_LINT := golangci-lint

dev-tools:
	@echo "🔧 Installing core tools..."
	@for pkg in \
		"github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest" \
		"golang.org/x/tools/cmd/goimports@latest" \
		"github.com/segmentio/golines@latest" \
		"mvdan.cc/gofumpt@latest" \
		"github.com/vektra/mockery/v2@latest" \
		"golang.org/x/tools/cmd/godoc@latest" \
		"golang.org/x/tools/cmd/deadcode@latest" \
		"github.com/mibk/dupl@latest" \
		"honnef.co/go/tools/cmd/staticcheck@latest"; do \
		name=$$(basename $$(echo $$pkg | cut -d@ -f1)); \
		printf "    %-28s" "$$name"; \
		$(GO) install $$pkg >/dev/null 2>&1 && echo "installed. ✅" || echo "failed. ❌"; \
	done; \
	echo "✅ Core tools installed"

install-tools: dev-tools
	@echo "📦 Installing optional tools..."
	@for pkg in \
		"github.com/kyoh86/richgo@latest" \
		"github.com/axw/gocov/gocov@latest" \
		"github.com/AlekSi/gocov-xml@latest" \
		"github.com/goreleaser/goreleaser@latest"; do \
		name=$$(basename $$(echo $$pkg | cut -d@ -f1)); \
		printf "    %-28s" "$$name"; \
		$(GO) install $$pkg >/dev/null 2>&1 && echo "installed. ✅" || echo "failed. ❌"; \
	done; \
	echo "✅ All tools installed"

# =============================================================================
# VALIDATION
# =============================================================================

validate: deps fmt vet lint test coverage-threshold
	@echo "✅ Full validation passed"

# =============================================================================
# CLEANUP
# =============================================================================

clean:
	@echo "🧹 Cleaning workspace..."
	@rm -rf $(BIN_DIR) $(COVERAGE_DIR) $(DIST_DIR)
	@rm -f *.prof *.test 2>/dev/null || true
	@find . -type f -name '*_test.go.*' -delete 2>/dev/null || true
	@echo "    artifacts and temporary files removed. ✅"

# =============================================================================
# DEFAULT
# =============================================================================
.DEFAULT_GOAL := help
