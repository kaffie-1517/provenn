.PHONY: up down migrate migrate-down seed build

# Start all services in detached mode
up:
	docker compose -f deploy/docker-compose.yml up --build -d

# Stop and remove all services and volumes
down:
	docker compose -f deploy/docker-compose.yml down -v

# Run database migrations (requires Postgres to be running via `make up`)
migrate:
	@docker run --rm \
		-v $(CURDIR)/internal/db/migrations:/migrations \
		migrate/migrate \
		-path=/migrations \
		-database "postgres://provenn:provenn@host.docker.internal:5432/provenn?sslmode=disable" up

# Roll back all migrations
migrate-down:
	@docker run --rm \
		-v $(CURDIR)/internal/db/migrations:/migrations \
		migrate/migrate \
		-path=/migrations \
		-database "postgres://provenn:provenn@host.docker.internal:5432/provenn?sslmode=disable" down -all

# Seed the database with demo data (stub — will be implemented in step 12)
seed:
	@echo "TODO: run seed script"


# Build Go binaries locally
build:
	go build -o api ./cmd/api
	go build -o worker ./cmd/worker
