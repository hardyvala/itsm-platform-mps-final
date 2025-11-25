# ITSM Platform - Graph-Driven DSL with MPS Code Generation

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         JetBrains MPS (Build Time)                          │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         DSL Definition                               │   │
│  │   dsl/apps/{service}/service.json                                   │   │
│  │   - Nodes (entities)                                                 │   │
│  │   - Properties (fields)                                              │   │
│  │   - Relations (no FK, just ID mapping)                              │   │
│  │   - Hooks (validations, rules, actions, triggers)                   │   │
│  │   - Events (NATS publish/subscribe)                                 │   │
│  │   - Graph (Apache AGE sync)                                         │   │
│  └──────────────────────────────┬──────────────────────────────────────┘   │
│                                 │                                           │
│                                 ▼                                           │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     MPS Code Generator                               │   │
│  │   - Parse DSL JSON                                                   │   │
│  │   - Generate main.go with DAL init                                  │   │
│  │   - Generate hook registrations from DSL                            │   │
│  │   - Generate HTTP routes for all nodes                              │   │
│  │   - Generate NATS subscriptions                                     │   │
│  └──────────────────────────────┬──────────────────────────────────────┘   │
│                                 │                                           │
│                                 ▼                                           │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     Generated Code                                   │   │
│  │   generated/{service}-service/main.go                               │   │
│  │   - Service bootstrap                                                │   │
│  │   - DAL initialization                                               │   │
│  │   - Hook executor setup                                              │   │
│  │   - HTTP handlers                                                    │   │
│  │   - Action/Trigger stubs (developer fills in)                       │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              SDK (Runtime)                                  │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐              │
│  │   Parser   │ │   Query    │ │    DAL     │ │   Hooks    │              │
│  │  (DSL→Go)  │ │  (JSON→SQL)│ │  (CRUD)    │ │ (Executor) │              │
│  └────────────┘ └────────────┘ └────────────┘ └────────────┘              │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Infrastructure                                     │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐               │
│  │   PostgreSQL    │ │      NATS       │ │   Apache AGE    │               │
│  │  (No FK keys)   │ │   JetStream     │ │   (Graph DB)    │               │
│  │                 │ │                 │ │                 │               │
│  │  tenant_xxx/    │ │ TICKET_EVENTS   │ │  Ticket nodes   │               │
│  │   tickets       │ │ CUSTOMER_EVENTS │ │  REPORTED_BY    │               │
│  │   comments      │ │ ASSET_EVENTS    │ │  edges          │               │
│  └─────────────────┘ └─────────────────┘ └─────────────────┘               │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Key Design Decisions

### 1. No Foreign Keys in Database

Relations are tracked via ID fields only. Composite operations handled in DAL:

```json
// DSL defines relation, but no FK created
"relations": [
  {
    "name": "customer",
    "type": "belongs_to",
    "target_service": "customer",
    "target_node": "Customer",
    "local_field": "customer_id",  // Just a UUID column
    "target_field": "id"
  }
]
```

DAL handles the lookup:
```go
// Fetching related data without JOINs
entities, _ := ticketDAL.Execute(ctx, tenantID, query.Query{
    From: "tickets",
    Relations: []query.RelationQuery{
        {Name: "customer", Select: []string{"id", "name", "email"}},
    },
})
```

### 2. Hooks Defined in DSL, Logic in Code

DSL declares **what** hooks exist, code implements **how**:

```json
// DSL declares hooks
"hooks": {
  "pre_create": {
    "enabled": true,
    "validations": [
      {"field": "subject", "rule": "min_length", "value": 5, "message": "Subject too short"}
    ]
  },
  "post_create": {
    "enabled": true,
    "actions": ["notify_customer", "auto_assign", "calculate_sla"]
  },
  "post_update": {
    "enabled": true,
    "triggers": [
      {"on_field_change": "status", "action": "notify_status_change"}
    ]
  }
}
```

MPS generates:
```go
// Generated: Hook executor setup
hookExecutor := hooks.NewExecutor(node)
hookExecutor.RegisterAction("notify_customer", notifyCustomer)
hookExecutor.RegisterAction("auto_assign", autoAssignTicket)
hookExecutor.RegisterTrigger("notify_status_change", notifyStatusChange)

// Developer implements:
func notifyCustomer(ctx context.Context, entity dal.Entity) error {
    // Your business logic here
    return nil
}
```

### 3. JSON Query from UI → SQL

UI sends JSON queries, SDK converts to SQL:

```json
// UI sends this JSON
{
  "from": "tickets",
  "where": [
    {"field": "status", "op": "eq", "value": "open"},
    {"field": "priority", "op": "in", "value": ["high", "critical"]}
  ],
  "order_by": [{"field": "created_at", "dir": "desc"}],
  "limit": 20,
  "offset": 0,
  "relations": [
    {"name": "customer", "select": ["id", "name"]}
  ]
}
```

SDK generates:
```sql
SELECT * FROM tenant_xxx.tickets 
WHERE tenant_id = $1 
  AND deleted_at IS NULL 
  AND status = $2 
  AND priority IN ($3, $4)
ORDER BY created_at DESC
LIMIT 20 OFFSET 0
```

## Directory Structure

```
itsm-platform/
├── dsl/
│   └── apps/
│       ├── ticket/
│       │   └── service.json      # Ticket + Comment nodes
│       ├── customer/
│       │   └── service.json      # Customer node
│       └── asset/
│           └── service.json      # Asset node
│
├── sdk/
│   ├── parser/
│   │   └── dsl.go               # DSL types and parser
│   ├── query/
│   │   └── builder.go           # JSON → SQL converter
│   ├── dal/
│   │   └── dal.go               # Generic CRUD (no FK)
│   └── hooks/
│       └── executor.go          # DSL hook executor
│
├── generated/                    # MPS generates these
│   └── ticket-service/
│       └── main.go              # Generated service code
│
└── services/                     # Final deployable (after adding business logic)
    ├── ticket-service/
    ├── customer-service/
    └── asset-service/
```

## DSL Schema Reference

### Node (Entity)

```json
{
  "name": "Ticket",
  "table": "tickets",
  "properties": [...],
  "indexes": [...],
  "dal": {
    "soft_delete": true,
    "optimistic_lock": true
  },
  "relations": [...],
  "hooks": {...},
  "graph": {...}
}
```

### Property

```json
{
  "name": "status",
  "type": "enum",
  "values": ["open", "in_progress", "resolved", "closed"],
  "default": "open",
  "indexed": true,
  "required": true
}
```

Types: `uuid`, `text`, `boolean`, `timestamp`, `integer`, `decimal`, `jsonb`, `enum`

### Relation (No FK)

```json
{
  "name": "customer",
  "type": "belongs_to",           // belongs_to, has_many, has_one
  "target_service": "customer",   // Cross-service
  "target_node": "Customer",
  "local_field": "customer_id",   // Just stores UUID
  "target_field": "id"
}
```

### Hooks

```json
{
  "pre_create": {
    "enabled": true,
    "validations": [
      {"field": "email", "rule": "email_format", "message": "Invalid email"}
    ]
  },
  "post_create": {
    "enabled": true,
    "actions": ["send_welcome_email"]
  },
  "pre_update": {
    "enabled": true,
    "rules": [
      {"condition": "old.status == 'closed'", "action": "reject", "message": "Cannot modify"}
    ]
  },
  "post_update": {
    "enabled": true,
    "triggers": [
      {"on_field_change": "status", "action": "notify_status_change"}
    ]
  },
  "pre_delete": {
    "enabled": true,
    "checks": ["has_no_dependencies"]
  },
  "post_delete": {
    "enabled": true,
    "actions": ["cleanup_related_data"]
  }
}
```

Validation rules: `required`, `min_length`, `max_length`, `email_format`, `regex`, `in`

### Events (NATS)

```json
{
  "stream": "TICKET_EVENTS",
  "publish": [
    {"event": "ticket.created", "subject": "ticket.{tenant_id}.ticket.created"}
  ],
  "subscribe": [
    {"subject": "customer.*.customer.deleted", "handler": "on_customer_deleted"}
  ]
}
```

### Graph (Apache AGE)

```json
{
  "label": "Ticket",
  "sync_properties": ["id", "tenant_id", "subject", "status"],
  "edges": [
    {"type": "REPORTED_BY", "to": "Customer", "via": "customer_id"}
  ]
}
```

## JetBrains MPS Integration

### MPS Language Definition

1. **Concept: ServiceGraph** (root)
   - metadata: Metadata
   - nodes: Node[]
   - events: Events

2. **Concept: Node**
   - name: string
   - table: string
   - properties: Property[]
   - relations: Relation[]
   - hooks: Hooks

3. **Concept: Hooks**
   - pre_create: HookConfig
   - post_create: HookConfig
   - ...

### MPS Generator Template

```
// Template: generate_service_main
<#macro generate_service_main graph>
package main

const (
    ServiceName = "${graph.metadata.service}"
    ServicePort = ${graph.metadata.port}
)

<#list graph.nodes as node>
var ${node.name?uncap_first}DAL *dal.DAL
</#list>

func main() {
    // ... bootstrap code
    
<#list graph.nodes as node>
    ${node.name?uncap_first}DAL = init${node.name}DAL(db, eventBus)
</#list>
}

<#list graph.nodes as node>
func init${node.name}DAL(db *pgxpool.Pool, eventBus dal.EventPublisher) *dal.DAL {
    node := graph.GetNode("${node.name}")
    hookExecutor := hooks.NewExecutor(node)
    
    <#list node.hooks.post_create.actions as action>
    hookExecutor.RegisterAction("${action}", ${action})
    </#list>
    
    <#list node.hooks.post_update.triggers as trigger>
    hookExecutor.RegisterTrigger("${trigger.action}", ${trigger.action})
    </#list>
    
    return dal.NewDAL(db, dalNode, ServiceName, hookExecutor, eventBus)
}
</#list>
</#macro>
```

## Adding a New Service

### Step 1: Create DSL

```json
// dsl/apps/inventory/service.json
{
  "version": "2.0",
  "kind": "ServiceGraph",
  "metadata": {
    "service": "inventory",
    "database": "inventory_db",
    "port": 8004
  },
  "nodes": [
    {
      "name": "Product",
      "table": "products",
      "properties": [...],
      "hooks": {
        "pre_create": {"enabled": true, "validations": [...]},
        "post_create": {"enabled": true, "actions": ["update_stock"]}
      }
    }
  ],
  "events": {...}
}
```

### Step 2: Run MPS Generator

```bash
# MPS generates the service scaffolding
mps-generate dsl/apps/inventory/service.json generated/inventory-service/
```

### Step 3: Implement Actions

```go
// In generated/inventory-service/main.go
// Find the action stubs and implement them:

func updateStock(ctx context.Context, entity dal.Entity) error {
    // Your business logic
    return nil
}
```

### Step 4: Deploy

```bash
go build -o inventory-service generated/inventory-service/
./inventory-service
```

## API Usage

### Query Endpoint (JSON from UI)

```bash
# Complex query
curl -X POST "http://localhost:8001/api/v1/tickets/query" \
  -H "X-Tenant-ID: tenant_abc" \
  -H "Content-Type: application/json" \
  -d '{
    "from": "tickets",
    "where": [
      {"field": "status", "op": "in", "value": ["open", "in_progress"]},
      {"field": "priority", "op": "eq", "value": "critical"}
    ],
    "order_by": [{"field": "created_at", "dir": "desc"}],
    "limit": 10,
    "relations": [
      {"name": "customer", "select": ["name", "email"]}
    ]
  }'
```

### Standard CRUD

```bash
# Create
curl -X POST "http://localhost:8001/api/v1/tickets" \
  -H "X-Tenant-ID: tenant_abc" \
  -d '{"subject": "Cannot login", "priority": "high", "customer_id": "uuid"}'

# Update
curl -X PATCH "http://localhost:8001/api/v1/tickets/{id}" \
  -H "X-Tenant-ID: tenant_abc" \
  -d '{"status": "in_progress"}'
```

## Graph Queries (for LLM/KB)

```cypher
-- Find ticket and all related entities
MATCH (t:Ticket {id: 'xxx'})-[r]-(related)
RETURN t, type(r), related

-- Find tickets by customer
MATCH (t:Ticket)-[:REPORTED_BY]->(c:Customer {email: 'user@example.com'})
RETURN t

-- Build knowledge context for LLM
MATCH (t:Ticket {id: 'xxx'})-[*1..3]-(context)
RETURN context
```
