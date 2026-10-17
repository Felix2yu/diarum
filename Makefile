.PHONY: help build dev run clean test test-cover test-coverage frontend backend docker version install-air test-static lint lint-strict fmt

# Get version from git
VERSION ?= $(shell git describe --dirty --always --tags --abbrev=7 2>/dev/null || echo "dev")
LDFLAGS := -X main.Version=$(VERSION)
AIR_VERSION ?= latest
AIR := $(CURDIR)/.tmp/bin/air
GOCACHE ?= $(CURDIR)/.tmp/go-build
COVERAGE_FILE ?= $(CURDIR)/.tmp/coverage.out
COVERAGE_THRESHOLD ?= 80

# Default target
help:
	@echo "Diarum Development Commands:"
	@echo "  make build      - Build both frontend and backend"
	@echo "  make dev        - Run in development mode"
	@echo "  make run        - Run the application"
	@echo "  make frontend   - Build frontend only"
	@echo "  make backend    - Build backend only"
	@echo "  make clean      - Clean build artifacts"
	@echo "  make test       - Run tests"
	@echo "  make test-cover - Run tests and enforce average function coverage >= $(COVERAGE_THRESHOLD)%"
	@echo "  make docker     - Build Docker image"
	@echo "  make version    - Show current version"

# Show version
version:
	@echo "Version: $(VERSION)"

# Build everything
build: frontend backend

# Build frontend
frontend:
	@echo "Building frontend..."
	cd web && pnpm install --frozen-lockfile && pnpm run build

# Build backend
backend: frontend
	@echo "Copying frontend build to embed location..."
	@mkdir -p internal/static/build
	@cp -r web/build/* internal/static/build/
	@echo "Building backend with version $(VERSION)..."
	go build -ldflags "$(LDFLAGS)" -o diarum ./cmd/diarum

# Development mode (requires running frontend and backend separately)
dev:
	@echo "Starting development mode..."
	@echo "Run 'make dev-frontend' in one terminal and 'make dev-backend' in another"

dev-frontend:
	@echo "Installing frontend dependencies..."
	@cd web && pnpm install --frozen-lockfile
	@echo "Starting frontend dev server..."
	cd web && pnpm run dev

dev-backend:
	@echo "Installing backend dependencies..."
	@go mod download
	@mkdir -p .tmp
	@$(MAKE) install-air
	@echo "Starting backend server with air..."
	exec $(AIR) -c .air.toml

install-air: $(AIR)

$(AIR):
	@echo "Installing air..."
	@mkdir -p .tmp/bin
	@GOBIN=$(CURDIR)/.tmp/bin go install github.com/air-verse/air@$(AIR_VERSION)

# Run the built application
run:
	./diarum serve

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f diarum
	rm -rf .tmp
	rm -rf web/build web/node_modules web/.svelte-kit
	rm -rf dist
	rm -rf bin

# Run tests
test-static:
	@mkdir -p internal/static/build
	@test -f internal/static/build/index.html || printf '<!doctype html><title>Diarum test static</title>\n' > internal/static/build/index.html

test: test-static
	@mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) go test ./...

test-cover test-coverage: test-static
	@mkdir -p $(dir $(COVERAGE_FILE)) $(GOCACHE)
	GOCACHE=$(GOCACHE) go test -coverprofile=$(COVERAGE_FILE) ./...
	GOCACHE=$(GOCACHE) go tool cover -func=$(COVERAGE_FILE)
	@coverage=$$(GOCACHE=$(GOCACHE) go tool cover -func=$(COVERAGE_FILE) | grep -v -E '\b(main|init)\b\s+[0-9]' | awk '$$1 != "total:" {gsub("%", "", $$NF); sum += $$NF; count++} END {if (count > 0) printf "%.1f", sum / count; else print "0.0"}'); \
	echo "Average function coverage: $$coverage%"; \
	awk "BEGIN { exit !($$coverage >= $(COVERAGE_THRESHOLD)) }" || (echo "Average function coverage $$coverage% is below $(COVERAGE_THRESHOLD)%" && exit 1)

# 静态检查：gofmt 只列出未格式化文件（不写盘），再跑 go vet。
lint:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "未格式化："; echo "$$out"; exit 1; fi
	go vet ./...

# 深度检查：在 lint 之上加 golangci-lint（存量告警未清零前会红，故不并入 lint）
lint-strict: lint
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run; else echo "  （未安装 golangci-lint，跳过）"; fi

fmt:
	gofmt -w $$(git ls-files '*.go')

# Build Docker image (Dockerfile only assembles; binary is built locally first)
docker:
	@echo "Building binary for Docker image..."
	$(MAKE) build
	@mkdir -p bin
	@cp diarum bin/diarum
	docker build -t diarum:latest .

# Install dependencies
deps:
	go mod download
	cd web && pnpm install --frozen-lockfile
