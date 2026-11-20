.PHONY: all build run test clean templ db-up db-down

export GOCACHE := /tmp/gocache
TEMPL := $(shell which templ 2>/dev/null || echo $(HOME)/go/bin/templ)

all: build

templ:
	@echo "Generating templ templates..."
	@$(TEMPL) generate

build:
	@echo "Building aura-forge..."
	@mkdir -p bin
	@GOCACHE=/tmp/gocache go build -o bin/aura-forge ./cmd/server

run:
	@echo "Starting aura-forge..."
	@GOCACHE=/tmp/gocache go run ./cmd/server

test:
	@echo "Running tests..."
	@GOCACHE=/tmp/gocache go test -v ./...

db-up:
	@docker compose up -d

db-down:
	@docker compose down

clean:
	@rm -rf bin/
