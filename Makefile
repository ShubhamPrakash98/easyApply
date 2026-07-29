.PHONY: dev-api dev-dashboard dev-extension migrate-up migrate-down migrate-new db-up db-down

DATABASE_URL ?= postgres://oneapply:oneapply@localhost:5432/oneapply?sslmode=disable
MIGRATIONS_DIR = backend/migrations

dev-api:
	cd backend && go run ./cmd/api

dev-dashboard:
	pnpm dev:dashboard

dev-extension:
	pnpm dev:extension

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

migrate-up:
	migrate -database "$(DATABASE_URL)" -path $(MIGRATIONS_DIR) up

migrate-down:
	migrate -database "$(DATABASE_URL)" -path $(MIGRATIONS_DIR) down 1

# usage: make migrate-new name=add_something
migrate-new:
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)
