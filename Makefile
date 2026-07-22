# local Postgres for dev/tests; the app runs on the host via `make run`.
DATABASE_URL ?= postgres://kova:kova@localhost:5433/kova?sslmode=disable

.PHONY: db db-stop run dev up down test test-db build sdk

db: ## start Postgres in the background
	docker compose up -d db

db-stop:
	docker compose stop db

run: ## run the server on the host against the compose DB
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/server

dev: db ## start DB then run the server on the host
	sleep 2 && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/server

up: ## build + run everything in Docker
	docker compose up --build

down:
	docker compose down

test: ## run all Go tests (DB-backed tests skip without TEST_DATABASE_URL)
	go test ./...

test-db: db ## run tests against the compose DB
	sleep 2 && TEST_DATABASE_URL="$(DATABASE_URL)" go test ./...

build:
	CGO_ENABLED=0 go build -o bin/kova ./cmd/server

sdk: ## rebuild the JS SDK and stage it into the server
	cd sdk && npm run build && cp dist/kova.global.js ../internal/api/static/kova.global.js
