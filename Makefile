-include .env
export

MIGRATIONS_DIR := migrations
DATABASE_SERVICE := postgres-shortener

# Exemplo: make help
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

# Exemplo: make infra/up
.PHONY: infra/up
infra/up:
	@docker compose up --wait $(DATABASE_SERVICE)

# Exemplo: make infra/down
.PHONY: infra/down
infra/down:
	@docker compose down

# Exemplo: make migration/new name=create_users
.PHONY: migration/new
migration/new:
	@test -n "$(name)" || (echo "name is required: make migration/new name=create_example"; exit 1)
	@goose -dir $(MIGRATIONS_DIR) create $(name) sql

# Exemplo: make migration/fix
.PHONY: migration/fix
migration/fix:
	@goose -dir $(MIGRATIONS_DIR) fix

# Exemplo: make migration/validate
.PHONY: migration/validate
migration/validate:
	@goose -dir $(MIGRATIONS_DIR) validate

# Exemplo: make migration/up
.PHONY: migration/up
migration/up:
	@goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" up

# Exemplo: make migration/down
.PHONY: migration/down
migration/down:
	@goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" down

# Exemplo: make migration/status
.PHONY: migration/status
migration/status:
	@goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" status

# Exemplo: make test
.PHONY: test
test:
	@go test ./... -count=1

# Exemplo: make run
.PHONY: run
run:
	@go run ./cmd/api
