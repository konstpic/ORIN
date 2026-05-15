SHELL := /bin/bash
BIN_DIR := ./bin
BINARY := $(BIN_DIR)/k8s-ui
PKG := github.com/k8s-ui/k8s-ui
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X $(PKG)/internal/config.Version=$(VERSION)

.PHONY: all build test lint tidy fmt vet clean docker frontend frontend-build run-all-in-one migrate-up migrate-down

all: build

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

build: $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/k8s-ui

test:
	go test ./... -race -count=1

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not installed; install from https://golangci-lint.run/"; exit 1; }
	golangci-lint run ./...

tidy:
	go mod tidy

fmt:
	gofmt -s -w .

vet:
	go vet ./...

clean:
	rm -rf $(BIN_DIR)

docker:
	docker build -t k8s-ui:$(VERSION) -t k8s-ui:dev -f Dockerfile .

frontend:
	cd web && npm install && npm run dev

frontend-build:
	cd web && npm install && npm run build

run-all-in-one: build
	$(BINARY) all-in-one

migrate-up: build
	$(BINARY) migrate up

migrate-down: build
	$(BINARY) migrate down
