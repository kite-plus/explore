GO       ?= go
BINARY   ?= explore
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     ?= $(shell git log -1 --format=%cI 2>/dev/null || echo unknown)
LDFLAGS  := -s -w \
	-X github.com/kite-plus/explore/internal/buildinfo.Version=$(VERSION) \
	-X github.com/kite-plus/explore/internal/buildinfo.Commit=$(COMMIT) \
	-X github.com/kite-plus/explore/internal/buildinfo.Date=$(DATE)

.PHONY: all build test test-race cover fmt vet lint check-imports check-tidy check tidy clean db-up db-down migrate docker web web-check web-test docker-web

all: check build

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/$(BINARY) ./cmd/explore

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

cover:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

LINT_VERSION ?= v2.13.2

lint:
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VERSION) run ./...

check-imports:
	@sh scripts/check-imports.sh

# Compares against the files before tidy, not HEAD, so uncommitted edits are
# not reported as untidiness.
check-tidy:
	@cp go.mod .go.mod.check && cp go.sum .go.sum.check
	@$(GO) mod tidy
	@if ! cmp -s go.mod .go.mod.check || ! cmp -s go.sum .go.sum.check; then \
		rm -f .go.mod.check .go.sum.check; \
		echo "go.mod or go.sum was not tidy; tidy has fixed them in place"; \
		exit 1; \
	fi
	@rm -f .go.mod.check .go.sum.check

check: fmt vet check-imports check-tidy lint test

tidy:
	$(GO) mod tidy

clean:
	rm -rf bin coverage.out

DEV_COMPOSE := docker compose -f deploy/docker-compose.dev.yaml
DEV_DATABASE_URL ?= postgres://explore:explore@127.0.0.1:5433/explore?sslmode=disable

db-up:
	$(DEV_COMPOSE) up -d --wait postgres

db-down:
	$(DEV_COMPOSE) down

migrate:
	EXPLORE_DATABASE_URL='$(DEV_DATABASE_URL)' $(GO) run ./cmd/explore migrate up

IMAGE ?= ghcr.io/kite-plus/explore

docker:
	docker build -f deploy/Dockerfile \
		--build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg DATE=$(DATE) \
		-t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

PNPM ?= pnpm

# The frontend needs Node, which a Go-only contributor should not have to
# install, so it has its own targets.
web:
	cd web && $(PNPM) install --frozen-lockfile && $(PNPM) build

web-check:
	cd web && $(PNPM) check

web-test: web
	cd web && $(PNPM) test

docker-web:
	docker build -t $(IMAGE)-web:$(VERSION) -t $(IMAGE)-web:latest web
