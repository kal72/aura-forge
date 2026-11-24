.PHONY: all build run test clean templ css db-up db-down docker-up docker-run docker-rebuild docker-down docker-logs migrate-up migrate-down migrate-status migrate-new

export GOCACHE := /tmp/gocache
TEMPL := $(shell which templ 2>/dev/null || echo $(HOME)/go/bin/templ)

all: build

templ:
	@echo "Generating templ templates..."
	@$(TEMPL) generate

css:
	@echo "Compiling pre-built Tailwind CSS to web/static/tailwind.css..."
	@npx --yes tailwindcss@3.4.17 -c tailwind.config.js -i web/static/tailwind.input.css -o web/static/tailwind.css --minify

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
	@docker compose up -d postgres

db-down:
	@docker compose stop postgres

docker-up:
	@docker compose up -d

docker-run: templ
	@docker compose up --build -d

docker-down:
	@docker compose down

docker-logs:
	@docker compose logs -f aura-forge

migrate-up:
	@GOCACHE=/tmp/gocache go run ./cmd/migrate up

migrate-down:
	@GOCACHE=/tmp/gocache go run ./cmd/migrate down

migrate-status:
	@GOCACHE=/tmp/gocache go run ./cmd/migrate status

migrate-new:
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-new NAME=deskripsi_migrasi"; exit 1; fi
	@GOCACHE=/tmp/gocache go run ./cmd/migrate create $(NAME)

clean:
	@rm -rf bin/
