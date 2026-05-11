.PHONY: build test lint vet e2e clean cover help

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

help:
	@echo "Available targets:"
	@echo "  build   - Build the easyinfra binary"
	@echo "  test    - Run unit tests with race detector"
	@echo "  lint    - Run golangci-lint"
	@echo "  vet     - Run go vet"
	@echo "  e2e     - Run end-to-end tests"
	@echo "  cover   - Generate coverage report"
	@echo "  clean   - Remove build artifacts"

build:
	go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o bin/easyinfra ./cmd/easyinfra

test:
	go test -race -cover ./...

lint:
	golangci-lint run

vet:
	go vet ./...

e2e:
	go test -tags=e2e -race ./test/e2e/...

cover:
	go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

clean:
	rm -rf bin/ dist/ coverage.out
