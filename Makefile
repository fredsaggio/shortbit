-include .env
export

MIGRATIONS_DIR := migrations
DATABASE_SERVICE := postgres-shortener

.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make infra/up"
	@echo "  make infra/down"
	@echo "  make migration/new name=create_example"
	@echo "  make migration/fix"
	@echo "  make migration/validate"
	@echo "  make migration/up"
	@echo "  make migration/down"
	@echo "  make migration/status"
	@echo "  make test"
	@echo "  make run"

.PHONY: infra/up
infra/up:
	@docker compose up --wait $(DATABASE_SERVICE)

.PHONY: infra/down
infra/down:
	@docker compose down

.PHONY: migration/new
migration/new:
	@test -n "$(name)" || (echo "name is required: make migration/new name=create_example"; exit 1)
	@goose -dir $(MIGRATIONS_DIR) create $(name) sql

.PHONY: migration/fix
migration/fix:
	@goose -dir $(MIGRATIONS_DIR) fix

.PHONY: migration/validate
migration/validate:
	@goose -dir $(MIGRATIONS_DIR) validate

.PHONY: migration/up
migration/up:
	@goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" up

.PHONY: migration/down
migration/down:
	@goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" down

.PHONY: migration/status
migration/status:
	@goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" status

.PHONY: test
test:
	@go test ./... -count=1

.PHONY: run
run:
	@go run ./cmd/api
