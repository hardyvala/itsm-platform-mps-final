# DAL Service

A standalone Database Access Layer service that handles all database operations for microservices via NATS messaging.

## Architecture

### Key Features
- **Centralized Database Access**: Single service managing all database operations
- **NATS-only Communication**: All inter-service communication via NATS (no HTTP)
- **DSL-based Schema Management**: Services define schemas using DSL
- **Tenant Isolation**: Schema-level isolation (tenant_xxx schemas)
- **Automatic Schema Generation**: Creates tables based on DSL definitions

### Components

1. **Main Service** (`main.go`)
   - NATS message handlers
   - Service registration
   - Event publishing

2. **Schema Manager** (`schema_manager.go`)
   - Tenant schema creation
   - Table generation from DSL
   - Index management

3. **Query Executor** (`query_executor.go`)
   - Query building and execution
   - CRUD operations
   - Relation handling

4. **Service Registry** (`registry.go`)
   - DSL registration
   - Service metadata management

5. **Migrator** (`migrator.go`)
   - Schema migration
   - DSL comparison
   - Automatic updates

6. **Client Library** (`client/client.go`)
   - Easy integration for services
   - Query builder
   - Request/response handling

## Usage

### Starting the DAL Service

```bash
# Set environment variables
export DATABASE_URL="postgres://user:pass@localhost/itsm"
export NATS_URL="nats://localhost:4222"

# Run the service
go run ./services/dal-service
```

### Service Integration

Services need to:
1. Register their DSL with the DAL service
2. Use the DAL client for all database operations

```go
import dalclient "itsm-platform/services/dal-service/client"

// Initialize client
dal := dalclient.NewClient(natsConn, "ticket-service")

// Register service DSL
dsl := loadDSL("./service.json")
dal.RegisterService("ticket-service", dsl)

// Create a tenant
dal.CreateTenant("tenant123")

// Query data
query := dalclient.NewQueryBuilder().
    Where("status", "eq", "open").
    OrderBy("created_at", true).
    Limit(10).
    Build()

results, err := dal.Query(ctx, "tenant123", "ticket", query)

// Create entity
ticket := map[string]interface{}{
    "title": "New Issue",
    "status": "open",
    "priority": "high",
}
result, err := dal.Create(ctx, "tenant123", "ticket", ticket)
```

## DSL Format

Services define their schema using DSL:

```json
{
  "version": "1.0",
  "kind": "ServiceGraph",
  "metadata": {
    "service": "ticket-service",
    "version": "1.0.0"
  },
  "nodes": [
    {
      "name": "ticket",
      "table": "tickets",
      "properties": [
        {
          "name": "title",
          "type": "string",
          "required": true,
          "max_length": 255
        },
        {
          "name": "status",
          "type": "enum",
          "values": ["open", "in_progress", "closed"],
          "required": true,
          "indexed": true
        },
        {
          "name": "priority",
          "type": "enum",
          "values": ["low", "medium", "high", "critical"],
          "indexed": true
        },
        {
          "name": "assignee_id",
          "type": "uuid",
          "indexed": true
        }
      ],
      "indexes": [
        {
          "name": "idx_status_priority",
          "fields": ["status", "priority"]
        }
      ],
      "dal": {
        "soft_delete": true,
        "optimistic_lock": true
      }
    }
  ],
  "edges": [
    {
      "name": "assignee",
      "from": "ticket",
      "to": "user",
      "type": "many_to_one",
      "local_key": "assignee_id",
      "external": true,
      "service": "user-service"
    }
  ]
}
```

## NATS Subjects

The DAL service listens on these NATS subjects:

- `dal.register` - Register service DSL
- `dal.{service}.{entity}.query` - Execute query
- `dal.{service}.{entity}.create` - Create entity
- `dal.{service}.{entity}.update` - Update entity
- `dal.{service}.{entity}.delete` - Delete entity
- `dal.{service}.{entity}.get` - Get by ID
- `dal.tenant.create` - Create tenant schema
- `dal.schema.migrate` - Run migrations

## Events

The DAL publishes events on entity changes:

- `{service}.{tenant_id}.{entity}.created`
- `{service}.{tenant_id}.{entity}.updated`
- `{service}.{tenant_id}.{entity}.deleted`

## Database Schema

Each tenant gets its own schema:
```
tenant_123/
  ├── tickets (from ticket-service)
  ├── users (from user-service)
  ├── customers (from customer-service)
  └── audit_log (system table)
```

## Features

### Tenant Isolation
- Complete schema separation per tenant
- No cross-tenant data access
- Automatic tenant_id filtering

### Soft Delete
- Optional per entity
- Automatic filtering of deleted records
- Preserves audit trail

### Optimistic Locking
- Version-based conflict detection
- Automatic version increment
- Prevents lost updates

### Query Features
- JSON-based query format
- Complex filtering
- Sorting and pagination
- Relation loading

### Schema Migration
- Automatic migration on DSL changes
- Safe column additions
- Index management
- Zero-downtime updates