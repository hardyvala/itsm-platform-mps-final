# UI Query JSON Format

The SDK accepts JSON queries from UI and converts them to SQL. This allows dynamic, flexible querying without writing SQL.

## Query Structure

```typescript
interface Query {
  select?: string[];         // Fields to return (default: all)
  from: string;              // Table name (from DSL node.table)
  where?: Condition[];       // Filter conditions
  order_by?: OrderClause[];  // Sorting
  limit?: number;            // Pagination
  offset?: number;           // Pagination
  relations?: RelationQuery[]; // Related data to fetch
}

interface Condition {
  field: string;
  op: string;      // eq, neq, gt, gte, lt, lte, like, ilike, in, not_in, is_null, is_not_null, between
  value?: any;
  or?: Condition[];  // OR group
  and?: Condition[]; // AND group
}

interface OrderClause {
  field: string;
  dir: "asc" | "desc";
}

interface RelationQuery {
  name: string;      // Relation name from DSL
  select?: string[]; // Fields from related entity
}
```

## Examples

### Simple Query

```json
{
  "from": "tickets",
  "where": [
    {"field": "status", "op": "eq", "value": "open"}
  ],
  "limit": 20
}
```

Generated SQL:
```sql
SELECT * FROM tenant_xxx.tickets 
WHERE tenant_id = $1 AND deleted_at IS NULL AND status = $2
LIMIT 20
```

### Multiple Conditions (AND)

```json
{
  "from": "tickets",
  "where": [
    {"field": "status", "op": "eq", "value": "open"},
    {"field": "priority", "op": "in", "value": ["high", "critical"]},
    {"field": "created_at", "op": "gte", "value": "2024-01-01T00:00:00Z"}
  ],
  "order_by": [{"field": "created_at", "dir": "desc"}],
  "limit": 50
}
```

Generated SQL:
```sql
SELECT * FROM tenant_xxx.tickets 
WHERE tenant_id = $1 
  AND deleted_at IS NULL 
  AND status = $2 
  AND priority IN ($3, $4)
  AND created_at >= $5
ORDER BY created_at DESC
LIMIT 50
```

### OR Conditions

```json
{
  "from": "tickets",
  "where": [
    {"field": "tenant_id", "op": "eq", "value": "xxx"},
    {
      "or": [
        {"field": "status", "op": "eq", "value": "open"},
        {"field": "priority", "op": "eq", "value": "critical"}
      ]
    }
  ]
}
```

Generated SQL:
```sql
SELECT * FROM tenant_xxx.tickets 
WHERE tenant_id = $1 
  AND deleted_at IS NULL 
  AND (status = $2 OR priority = $3)
```

### Nested AND/OR

```json
{
  "from": "tickets",
  "where": [
    {
      "or": [
        {
          "and": [
            {"field": "status", "op": "eq", "value": "open"},
            {"field": "priority", "op": "eq", "value": "high"}
          ]
        },
        {
          "and": [
            {"field": "status", "op": "eq", "value": "in_progress"},
            {"field": "priority", "op": "eq", "value": "critical"}
          ]
        }
      ]
    }
  ]
}
```

Generated SQL:
```sql
SELECT * FROM tenant_xxx.tickets 
WHERE tenant_id = $1 
  AND deleted_at IS NULL 
  AND (
    (status = $2 AND priority = $3) 
    OR 
    (status = $4 AND priority = $5)
  )
```

### Select Specific Fields

```json
{
  "select": ["id", "subject", "status", "created_at"],
  "from": "tickets",
  "where": [
    {"field": "status", "op": "neq", "value": "closed"}
  ]
}
```

### Text Search (LIKE/ILIKE)

```json
{
  "from": "tickets",
  "where": [
    {"field": "subject", "op": "ilike", "value": "%login%"}
  ]
}
```

### NULL Checks

```json
{
  "from": "tickets",
  "where": [
    {"field": "assigned_to", "op": "is_null"}
  ]
}
```

```json
{
  "from": "tickets",
  "where": [
    {"field": "resolved_at", "op": "is_not_null"}
  ]
}
```

### Date Range (BETWEEN)

```json
{
  "from": "tickets",
  "where": [
    {
      "field": "created_at", 
      "op": "between", 
      "value": ["2024-01-01T00:00:00Z", "2024-12-31T23:59:59Z"]
    }
  ]
}
```

### With Relations (No JOIN, separate fetch)

```json
{
  "from": "tickets",
  "where": [
    {"field": "id", "op": "eq", "value": "ticket-uuid"}
  ],
  "relations": [
    {"name": "customer", "select": ["id", "name", "email"]},
    {"name": "comments", "select": ["id", "body", "created_at"]}
  ]
}
```

Response includes related data:
```json
{
  "data": [
    {
      "id": "ticket-uuid",
      "subject": "Cannot login",
      "status": "open",
      "customer_id": "customer-uuid",
      "customer": {
        "id": "customer-uuid",
        "name": "John Doe",
        "email": "john@example.com"
      },
      "comments": [
        {"id": "comment-1", "body": "Trying to help", "created_at": "..."},
        {"id": "comment-2", "body": "Fixed", "created_at": "..."}
      ]
    }
  ],
  "total": 1
}
```

### Pagination

```json
{
  "from": "tickets",
  "where": [
    {"field": "status", "op": "eq", "value": "open"}
  ],
  "order_by": [{"field": "created_at", "dir": "desc"}],
  "limit": 10,
  "offset": 20
}
```

### Complex Dashboard Query

```json
{
  "from": "tickets",
  "where": [
    {"field": "status", "op": "in", "value": ["open", "in_progress"]},
    {"field": "assigned_to", "op": "eq", "value": "agent-uuid"},
    {"field": "due_date", "op": "lt", "value": "2024-12-01T00:00:00Z"},
    {
      "or": [
        {"field": "priority", "op": "eq", "value": "critical"},
        {
          "and": [
            {"field": "priority", "op": "eq", "value": "high"},
            {"field": "created_at", "op": "lt", "value": "2024-11-01T00:00:00Z"}
          ]
        }
      ]
    }
  ],
  "order_by": [
    {"field": "priority", "dir": "desc"},
    {"field": "created_at", "dir": "asc"}
  ],
  "limit": 50,
  "relations": [
    {"name": "customer", "select": ["name"]}
  ]
}
```

## Operators Reference

| Operator | SQL | Example |
|----------|-----|---------|
| `eq` | `=` | `{"field": "status", "op": "eq", "value": "open"}` |
| `neq` | `!=` | `{"field": "status", "op": "neq", "value": "closed"}` |
| `gt` | `>` | `{"field": "created_at", "op": "gt", "value": "2024-01-01"}` |
| `gte` | `>=` | `{"field": "priority", "op": "gte", "value": 3}` |
| `lt` | `<` | `{"field": "due_date", "op": "lt", "value": "2024-12-01"}` |
| `lte` | `<=` | `{"field": "count", "op": "lte", "value": 100}` |
| `like` | `LIKE` | `{"field": "name", "op": "like", "value": "%john%"}` |
| `ilike` | `ILIKE` | `{"field": "email", "op": "ilike", "value": "%@gmail.com"}` |
| `in` | `IN` | `{"field": "status", "op": "in", "value": ["a", "b"]}` |
| `not_in` | `NOT IN` | `{"field": "type", "op": "not_in", "value": ["x", "y"]}` |
| `is_null` | `IS NULL` | `{"field": "deleted_at", "op": "is_null"}` |
| `is_not_null` | `IS NOT NULL` | `{"field": "assigned_to", "op": "is_not_null"}` |
| `between` | `BETWEEN` | `{"field": "date", "op": "between", "value": ["a", "b"]}` |

## Security Notes

1. **Field validation**: Only fields defined in DSL are allowed
2. **Tenant isolation**: `tenant_id` filter always added automatically
3. **Soft delete**: `deleted_at IS NULL` added automatically if enabled
4. **SQL injection**: Parameterized queries used for all values
