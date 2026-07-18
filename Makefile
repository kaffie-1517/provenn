.PHONY: up down migrate seed build

# Start all services in detached mode
up:
	docker compose -f deploy/docker-compose.yml up --build -d

# Stop and remove all services and volumes
down:
	docker compose -f deploy/docker-compose.yml down -v

# Run database migrations (stub — will use golang-migrate in step 2)
migrate:
	@echo "TODO: run golang-migrate against DATABASE_URL"

# Seed the database with demo data (stub — will be implemented in step 12)
seed:
	@echo "TODO: run seed script"

# Build Go binaries locally
build:
	go build -o api ./cmd/api
	go build -o worker ./cmd/worker
