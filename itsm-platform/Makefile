# =============================================================================
# ITSM Platform - Build & Code Generation
# =============================================================================

.PHONY: generate generate-all build clean

# Build the code generator
codegen:
	@echo "Building code generator..."
	@go build -o bin/codegen ./cmd/codegen

# Generate single service
# Usage: make generate SERVICE=ticket
generate: codegen
	@echo "Generating $(SERVICE)-service..."
	@./bin/codegen dsl/apps/$(SERVICE)/service.json generated/$(SERVICE)-service
	@echo "✓ Generated $(SERVICE)-service"

# Generate ALL services from DSL
generate-all: codegen
	@echo "=== Generating all services from DSL ==="
	@for service in ticket customer asset; do \
		echo "Generating $$service-service..."; \
		./bin/codegen dsl/apps/$$service/service.json generated/$$service-service; \
	done
	@echo "=== All services generated ==="

# Build all services
build: generate-all
	@echo "Building services..."
	@for service in ticket customer asset; do \
		echo "Building $$service-service..."; \
		go build -o bin/$$service-service ./generated/$$service-service; \
	done
	@echo "✓ All services built"

# Clean generated files
clean:
	@rm -rf generated/
	@rm -rf bin/
	@echo "✓ Cleaned"

# Show what would be generated
dry-run:
	@echo "Services to generate:"
	@for dsl in dsl/apps/*/service.json; do \
		service=$$(dirname $$dsl | xargs basename); \
		echo "  - $$service-service (from $$dsl)"; \
	done

# Development: watch DSL and regenerate
watch:
	@echo "Watching DSL files for changes..."
	@while true; do \
		inotifywait -q -e modify dsl/apps/*/service.json; \
		make generate-all; \
	done
