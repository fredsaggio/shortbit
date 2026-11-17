-include .env
export

BACKEND_DIR := backend
FRONTEND_DIR := frontend
MIGRATIONS_DIR := $(BACKEND_DIR)/migrations
DATABASE_SERVICE := postgres-shortener

# Exemplo: make help
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make up"
	@echo "  make infra/up"
	@echo "  make infra/down"
	@echo "  make migration/new name=create_example"
	@echo "  make migration/fix"
	@echo "  make migration/validate"
	@echo "  make migration/up"
	@echo "  make migration/down"
	@echo "  make migration/status"
	@echo "  make test"
	@echo "  make test/integration"
	@echo "  make run"
	@echo "  make worker"
	@echo "  make geoip"
	@echo "  make front/install"
	@echo "  make front/dev"
	@echo "  make front/build"
	@echo "  make front/lint"
	@echo "  make front/test"

# Exemplo: make infra/up
.PHONY: infra/up
infra/up:
	@docker compose up --wait $(DATABASE_SERVICE)

# Sobe tudo em contêineres: PostgreSQL, Redis, RabbitMQ, API (com o front) e worker.
# Exemplo: make up
.PHONY: up
up:
	@docker compose up -d --build --wait

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
# O goose validate lê todo arquivo da pasta e trata o embed.go como uma
# migration em Go sem versão. Valida uma cópia só com os .sql.
migration/validate:
	@tmp=$$(mktemp -d) && cp $(MIGRATIONS_DIR)/*.sql "$$tmp" && goose -dir "$$tmp" validate; status=$$?; rm -rf "$$tmp"; exit $$status

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
	@go -C $(BACKEND_DIR) test ./... -count=1

# Exemplo: make test/integration
.PHONY: test/integration
test/integration:
	@go -C $(BACKEND_DIR) test -tags=integration ./internal/db ./internal/repositories ./internal/integration ./internal/messaging ./internal/server ./internal/cache -v -count=1

# Exemplo: make run
.PHONY: run
run:
	@go -C $(BACKEND_DIR) run ./cmd/api

# Exemplo: make worker
.PHONY: worker
worker:
	@go -C $(BACKEND_DIR) run ./cmd/worker

# Baixa a base IP->país da DB-IP para backend/.geoip (ADR 0007). Aponte
# GEOIP_DB_PATH para o arquivo no .env para os cliques terem país.
# Exemplo: make geoip
.PHONY: geoip
geoip:
	@sh $(BACKEND_DIR)/scripts/download-geoip.sh $(BACKEND_DIR)/.geoip/dbip-country-lite.mmdb

# Exemplo: make front/install
.PHONY: front/install
front/install:
	@npm --prefix $(FRONTEND_DIR) ci

# Exemplo: make front/dev
.PHONY: front/dev
front/dev:
	@npm --prefix $(FRONTEND_DIR) run dev

# Exemplo: make front/build
.PHONY: front/build
front/build:
	@npm --prefix $(FRONTEND_DIR) run build

# Exemplo: make front/lint
.PHONY: front/lint
front/lint:
	@npm --prefix $(FRONTEND_DIR) run lint

# Exemplo: make front/test
.PHONY: front/test
front/test:
	@npm --prefix $(FRONTEND_DIR) run test
