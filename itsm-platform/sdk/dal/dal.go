package dal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"itsm-platform/sdk/query"
)

// Entity represents a generic entity
type Entity map[string]interface{}

// DAL handles all database operations for a node
type DAL struct {
	db           *pgxpool.Pool
	node         *Node           // From DSL
	queryBuilder *query.Builder
	hooks        HookExecutor
	eventBus     EventPublisher
	service      string
}

// Node represents entity metadata from DSL
type Node struct {
	Name       string
	Table      string
	Properties []Property
	Relations  []Relation
	DALConfig  DALConfig
}

type Property struct {
	Name     string
	Type     string
	Required bool
	Default  string
}

type Relation struct {
	Name          string // e.g., "customer"
	Type          string // belongs_to, has_many, has_one
	TargetService string // e.g., "customer"
	TargetNode    string // e.g., "Customer"
	LocalField    string // e.g., "customer_id"
	TargetField   string // e.g., "id"
}

type DALConfig struct {
	SoftDelete     bool
	OptimisticLock bool
}

// HookExecutor interface - implemented by generated code
type HookExecutor interface {
	PreCreate(ctx context.Context, entity Entity) error
	PostCreate(ctx context.Context, entity Entity) error
	PreUpdate(ctx context.Context, old, new Entity) error
	PostUpdate(ctx context.Context, old, new Entity) error
	PreDelete(ctx context.Context, entity Entity) error
	PostDelete(ctx context.Context, entity Entity) error
}

// EventPublisher interface for NATS
type EventPublisher interface {
	Publish(ctx context.Context, subject string, data interface{}) error
}

// NewDAL creates a new DAL instance
func NewDAL(db *pgxpool.Pool, node *Node, service string, hooks HookExecutor, eventBus EventPublisher) *DAL {
	// Build property list for query builder
	props := make([]string, len(node.Properties))
	for i, p := range node.Properties {
		props[i] = p.Name
	}

	return &DAL{
		db:       db,
		node:     node,
		hooks:    hooks,
		eventBus: eventBus,
		service:  service,
	}
}

// Execute runs a JSON query from UI
func (d *DAL) Execute(ctx context.Context, tenantID string, q query.Query) ([]Entity, int64, error) {
	schema := fmt.Sprintf("tenant_%s", tenantID)
	
	// Create query builder with schema
	props := make([]string, len(d.node.Properties))
	for i, p := range d.node.Properties {
		props[i] = p.Name
	}
	builder := query.NewBuilder(schema, props)

	// Add tenant_id filter automatically
	q.Where = append([]query.Condition{
		{Field: "tenant_id", Operator: "eq", Value: tenantID},
	}, q.Where...)

	// Add soft delete filter
	if d.node.DALConfig.SoftDelete {
		q.Where = append(q.Where, query.Condition{
			Field: "deleted_at", Operator: "is_null",
		})
	}

	// Build SQL
	sql, params, err := builder.Build(q)
	if err != nil {
		return nil, 0, fmt.Errorf("query build failed: %w", err)
	}

	// Execute query
	rows, err := d.db.Query(ctx, sql, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	entities, err := d.scanRows(rows)
	if err != nil {
		return nil, 0, err
	}

	// Get total count (without limit/offset)
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s WHERE tenant_id = $1", schema, d.node.Table)
	if d.node.DALConfig.SoftDelete {
		countSQL += " AND deleted_at IS NULL"
	}
	var total int64
	d.db.QueryRow(ctx, countSQL, tenantID).Scan(&total)

	// Fetch relations if requested
	if len(q.Relations) > 0 {
		entities, err = d.fetchRelations(ctx, tenantID, entities, q.Relations)
		if err != nil {
			return nil, 0, err
		}
	}

	return entities, total, nil
}

// Create inserts a new entity
func (d *DAL) Create(ctx context.Context, tenantID string, data Entity) (Entity, error) {
	schema := fmt.Sprintf("tenant_%s", tenantID)

	// Set defaults
	data["id"] = uuid.New().String()
	data["tenant_id"] = tenantID
	data["created_at"] = time.Now().UTC()
	data["updated_at"] = time.Now().UTC()

	if d.node.DALConfig.OptimisticLock {
		data["version"] = 1
	}

	// Pre-hook
	if d.hooks != nil {
		if err := d.hooks.PreCreate(ctx, data); err != nil {
			return nil, fmt.Errorf("pre-create hook failed: %w", err)
		}
	}

	// Build query
	props := make([]string, len(d.node.Properties))
	for i, p := range d.node.Properties {
		props[i] = p.Name
	}
	builder := query.NewBuilder(schema, props)

	sql, params, err := builder.BuildInsert(d.node.Table, data)
	if err != nil {
		return nil, err
	}

	// Execute
	row := d.db.QueryRow(ctx, sql, params...)
	result, err := d.scanRow(row)
	if err != nil {
		return nil, fmt.Errorf("create failed: %w", err)
	}

	// Post-hook
	if d.hooks != nil {
		if err := d.hooks.PostCreate(ctx, result); err != nil {
			return nil, fmt.Errorf("post-create hook failed: %w", err)
		}
	}

	// Publish event
	if d.eventBus != nil {
		subject := fmt.Sprintf("%s.%s.%s.created", d.service, tenantID, d.node.Table)
		d.eventBus.Publish(ctx, subject, result)
	}

	return result, nil
}

// GetByID retrieves an entity by ID
func (d *DAL) GetByID(ctx context.Context, tenantID, id string) (Entity, error) {
	schema := fmt.Sprintf("tenant_%s", tenantID)

	sql := fmt.Sprintf("SELECT * FROM %s.%s WHERE id = $1 AND tenant_id = $2",
		schema, d.node.Table)

	if d.node.DALConfig.SoftDelete {
		sql += " AND deleted_at IS NULL"
	}

	row := d.db.QueryRow(ctx, sql, id, tenantID)
	return d.scanRow(row)
}

// Update modifies an entity
func (d *DAL) Update(ctx context.Context, tenantID, id string, data Entity) (Entity, error) {
	// Get existing
	old, err := d.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("entity not found: %w", err)
	}

	data["updated_at"] = time.Now().UTC()

	// Pre-hook
	if d.hooks != nil {
		if err := d.hooks.PreUpdate(ctx, old, data); err != nil {
			return nil, fmt.Errorf("pre-update hook failed: %w", err)
		}
	}

	schema := fmt.Sprintf("tenant_%s", tenantID)
	props := make([]string, len(d.node.Properties))
	for i, p := range d.node.Properties {
		props[i] = p.Name
	}
	builder := query.NewBuilder(schema, props)

	sql, params, err := builder.BuildUpdate(d.node.Table, id, data)
	if err != nil {
		return nil, err
	}

	// Add optimistic lock check
	if d.node.DALConfig.OptimisticLock {
		if oldVersion, ok := old["version"].(int); ok {
			sql = fmt.Sprintf("%s AND version = %d", sql, oldVersion)
			data["version"] = oldVersion + 1
		}
	}

	row := d.db.QueryRow(ctx, sql, params...)
	result, err := d.scanRow(row)
	if err != nil {
		if err == pgx.ErrNoRows && d.node.DALConfig.OptimisticLock {
			return nil, fmt.Errorf("optimistic lock conflict")
		}
		return nil, fmt.Errorf("update failed: %w", err)
	}

	// Post-hook
	if d.hooks != nil {
		if err := d.hooks.PostUpdate(ctx, old, result); err != nil {
			return nil, fmt.Errorf("post-update hook failed: %w", err)
		}
	}

	// Publish event
	if d.eventBus != nil {
		subject := fmt.Sprintf("%s.%s.%s.updated", d.service, tenantID, d.node.Table)
		d.eventBus.Publish(ctx, subject, result)
	}

	return result, nil
}

// Delete removes an entity
func (d *DAL) Delete(ctx context.Context, tenantID, id string) error {
	entity, err := d.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}

	// Pre-hook
	if d.hooks != nil {
		if err := d.hooks.PreDelete(ctx, entity); err != nil {
			return fmt.Errorf("pre-delete hook failed: %w", err)
		}
	}

	schema := fmt.Sprintf("tenant_%s", tenantID)
	props := make([]string, len(d.node.Properties))
	for i, p := range d.node.Properties {
		props[i] = p.Name
	}
	builder := query.NewBuilder(schema, props)

	sql, params := builder.BuildDelete(d.node.Table, id, d.node.DALConfig.SoftDelete)
	_, err = d.db.Exec(ctx, sql, params...)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	// Post-hook
	if d.hooks != nil {
		if err := d.hooks.PostDelete(ctx, entity); err != nil {
			return fmt.Errorf("post-delete hook failed: %w", err)
		}
	}

	// Publish event
	if d.eventBus != nil {
		subject := fmt.Sprintf("%s.%s.%s.deleted", d.service, tenantID, d.node.Table)
		d.eventBus.Publish(ctx, subject, entity)
	}

	return nil
}

// fetchRelations fetches related entities (no FK, just ID lookup)
func (d *DAL) fetchRelations(ctx context.Context, tenantID string, entities []Entity, relations []query.RelationQuery) ([]Entity, error) {
	for _, relQuery := range relations {
		// Find relation definition
		var rel *Relation
		for i := range d.node.Relations {
			if d.node.Relations[i].Name == relQuery.Name {
				rel = &d.node.Relations[i]
				break
			}
		}
		if rel == nil {
			continue
		}

		// Collect IDs to fetch
		ids := make([]interface{}, 0, len(entities))
		for _, e := range entities {
			if id, ok := e[rel.LocalField]; ok && id != nil {
				ids = append(ids, id)
			}
		}

		if len(ids) == 0 {
			continue
		}

		// Fetch related entities
		// For cross-service relations, this would call the other service's API
		// For same-service, query directly
		if rel.TargetService == d.service {
			related, err := d.fetchRelatedEntities(ctx, tenantID, rel, ids)
			if err != nil {
				return nil, err
			}

			// Map back to entities
			relatedMap := make(map[string]Entity)
			for _, r := range related {
				if id, ok := r[rel.TargetField].(string); ok {
					relatedMap[id] = r
				}
			}

			for i := range entities {
				if localID, ok := entities[i][rel.LocalField].(string); ok {
					if relEntity, found := relatedMap[localID]; found {
						entities[i][rel.Name] = relEntity
					}
				}
			}
		} else {
			// Cross-service: store IDs, let client resolve or use service mesh
			// Or implement service-to-service call here
			for i := range entities {
				entities[i][rel.Name+"_pending"] = true
			}
		}
	}

	return entities, nil
}

// fetchRelatedEntities fetches entities from related table
func (d *DAL) fetchRelatedEntities(ctx context.Context, tenantID string, rel *Relation, ids []interface{}) ([]Entity, error) {
	schema := fmt.Sprintf("tenant_%s", tenantID)

	// Build IN clause
	placeholders := make([]string, len(ids))
	for i := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
	}

	// Note: We need target table name - this should come from DSL registry
	// For now, convert TargetNode to table name
	targetTable := toSnakeCase(rel.TargetNode) + "s"

	sql := fmt.Sprintf("SELECT * FROM %s.%s WHERE tenant_id = $1 AND %s IN (%s)",
		schema, targetTable, rel.TargetField, strings.Join(placeholders, ", "))

	params := []interface{}{tenantID}
	params = append(params, ids...)

	rows, err := d.db.Query(ctx, sql, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return d.scanRows(rows)
}

// scanRow scans a single row into Entity
func (d *DAL) scanRow(row pgx.Row) (Entity, error) {
	columns := make([]interface{}, len(d.node.Properties))
	columnPtrs := make([]interface{}, len(d.node.Properties))

	for i := range columns {
		columnPtrs[i] = &columns[i]
	}

	if err := row.Scan(columnPtrs...); err != nil {
		return nil, err
	}

	entity := make(Entity)
	for i, prop := range d.node.Properties {
		entity[prop.Name] = columns[i]
	}

	return entity, nil
}

// scanRows scans multiple rows
func (d *DAL) scanRows(rows pgx.Rows) ([]Entity, error) {
	var entities []Entity
	columns := rows.FieldDescriptions()

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		entity := make(Entity)
		for i, col := range columns {
			entity[string(col.Name)] = values[i]
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

// toSnakeCase converts CamelCase to snake_case
func toSnakeCase(s string) string {
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, byte(c+32))
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}
