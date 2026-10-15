VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: test lint build run

test:
	go test -race ./...

lint:
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }
	go vet ./...

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/ahp-matrix ./cmd/ahp-matrix

run: build
	./bin/ahp-matrix
