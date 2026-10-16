VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: test lint build run web web-check e2e snapshot

test:
	go test -race ./...

lint:
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }
	go vet ./...

# The UI is embedded at build time: run `make web` first (or `make run`).
build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/ahp-matrix ./cmd/ahp-matrix

run: web build
	./bin/ahp-matrix

web: web/node_modules
	cd web && npm run build

web/node_modules: web/package-lock.json
	cd web && npm ci
	@touch web/node_modules

web-check: web/node_modules
	cd web && npm run check && npm test

e2e: web/node_modules
	cd web && npm run e2e

snapshot:
	go run github.com/goreleaser/goreleaser/v2@v2.18.3 release --snapshot --clean
