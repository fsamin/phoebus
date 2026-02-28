VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BINARY   := phoebus
LDFLAGS  := -s -w \
	-X github.com/fsamin/phoebus/internal/version.Version=$(VERSION) \
	-X github.com/fsamin/phoebus/internal/version.Commit=$(COMMIT) \
	-X github.com/fsamin/phoebus/internal/version.Date=$(DATE)

.PHONY: all build frontend backend clean test lint docker e2e e2e-down

all: build

## Build the full application (frontend + backend in a single binary)
build: frontend backend

## Build the React frontend
frontend:
	@echo "→ Building frontend…"
	cd frontend && npm ci --silent && npm run build
	@rm -rf internal/ui/dist && cp -r frontend/dist internal/ui/dist

## Build the Go binary (expects frontend/dist to exist)
backend:
	@echo "→ Building backend…"
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o bin/$(BINARY) ./cmd/phoebus

## Run all tests
test:
	go test -race -count=1 ./...

## Run tests with coverage
cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

## Remove build artifacts
clean:
	rm -rf bin/ frontend/dist coverage.out

## Build Docker image
docker:
	docker build -t phoebus:$(VERSION) .

## Run E2E tests (Playwright in Docker)
e2e:
	docker compose -p phoebus-e2e -f docker-compose.e2e.yml up -d --build db-e2e minio-e2e
	docker compose -p phoebus-e2e -f docker-compose.e2e.yml --profile init run --rm minio-init
	docker compose -p phoebus-e2e -f docker-compose.e2e.yml up --build --abort-on-container-exit --exit-code-from playwright phoebus-e2e playwright
	@docker compose -p phoebus-e2e -f docker-compose.e2e.yml down -v 2>/dev/null || true

## Stop E2E environment
e2e-down:
	docker compose -p phoebus-e2e -f docker-compose.e2e.yml down -v
