.PHONY: help build up down logs clean

# Default target
help:
	@echo "ITSM Platform - DAL Service Management"
	@echo ""
	@echo "Available targets:"
	@echo "  make up          - Start DAL service with NATS cluster and PostgreSQL"
	@echo "  make down        - Stop all containers"
	@echo "  make logs        - Show logs for all services"
	@echo "  make clean       - Stop containers and remove volumes"

# Start core infrastructure (PostgreSQL, NATS cluster, DAL service)
up:
	@echo "Starting DAL service with infrastructure..."
	docker compose up -d postgres nats-1 nats-2 nats-3 dal-service
	@echo "Services are running!"
	@echo "NATS: nats://localhost:4222"
	@echo "PostgreSQL: localhost:5432"

# Stop all containers
down:
	@echo "Stopping all containers..."
	docker compose down

# Clean everything (including volumes)
clean:
	@echo "Cleaning up everything..."
	docker compose down -v
	@echo "Removed containers and volumes"

# Show logs
logs:
	docker compose logs -f

# DAL service logs
dal-logs:
	docker compose logs -f dal-service

# NATS cluster logs
nats-logs:
	docker compose logs -f nats-1 nats-2 nats-3

# PostgreSQL logs
db-logs:
	docker compose logs -f postgres

# Health check
health:
	@echo "Checking service health..."
	@docker compose ps