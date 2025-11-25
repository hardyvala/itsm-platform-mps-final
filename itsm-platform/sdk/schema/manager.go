package schema

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"itsm-platform/sdk/dsl"
)

// Manager handles schema operations based on DSL
type Manager struct {
	db    *pgxpool.Pool
	graph *dsl.ServiceGraph
}

func NewManager(db *pgxpool.Pool, graph *dsl.ServiceGraph) *Manager {
	return &Manager{db: db, graph: graph}
}

// CreateTenantSchema creates schema for a new tenant
func (m *Manager) CreateTenantSchema(ctx context.Context, tenantID string) error {
	schemaName := fmt.Sprintf("tenant_%s", tenantID)

	// Create schema
	if _, err := m.db.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schemaName)); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Create tables for each node
	for _, node := range m.graph.Nodes {
		ddl := m.generateTableDDL(node, schemaName)
		if _, err := m.db.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("failed to create table %s: %w", node.Table, err)
		}

		// Create indexes
		for _, idx := range node.Indexes {
			indexDDL := m.generateIndexDDL(idx, node.Table, schemaName)
			if _, err := m.db.Exec(ctx, indexDDL); err != nil {
				return fmt.Errorf("failed to create index %s: %w", idx.Name, err)
			}
		}
	}

	// Add foreign keys for local edges
	for _, edge := range m.graph.Edges {
		if edge.External == nil && edge.ForeignKey != nil {
			fkDDL := m.generateForeignKeyDDL(edge, schemaName)
			if _, err := m.db.Exec(ctx, fkDDL); err != nil {
				return fmt.Errorf("failed to create FK for edge %s: %w", edge.Name, err)
			}
		}
	}

	return nil
}

// generateTableDDL generates CREATE TABLE statement from Node
func (m *Manager) generateTableDDL(node dsl.Node, schema string) string {
	var columns []string
	var primaryKey string

	for _, prop := range node.Properties {
		col := m.propertyToColumn(prop)
		columns = append(columns, col)

		if prop.Primary {
			primaryKey = prop.Name
		}
	}

	// Add version column for optimistic locking
	if node.DAL.OptimisticLock {
		columns = append(columns, "version INTEGER NOT NULL DEFAULT 1")
	}

	ddl := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.%s (
    %s,
    PRIMARY KEY (%s)
)`, schema, node.Table, strings.Join(columns, ",\n    "), primaryKey)

	return ddl
}

// propertyToColumn converts DSL property to SQL column definition
func (m *Manager) propertyToColumn(prop dsl.Property) string {
	var sqlType string
	var constraints []string

	switch prop.Type {
	case "uuid":
		sqlType = "UUID"
		if prop.Primary {
			constraints = append(constraints, "DEFAULT gen_random_uuid()")
		}
	case "text":
		if prop.MaxLength > 0 {
			sqlType = fmt.Sprintf("VARCHAR(%d)", prop.MaxLength)
		} else {
			sqlType = "TEXT"
		}
	case "boolean":
		sqlType = "BOOLEAN"
	case "timestamp":
		sqlType = "TIMESTAMPTZ"
	case "integer":
		sqlType = "INTEGER"
	case "decimal":
		if prop.Precision > 0 {
			sqlType = fmt.Sprintf("DECIMAL(%d,%d)", prop.Precision, prop.Scale)
		} else {
			sqlType = "DECIMAL(10,2)"
		}
	case "jsonb":
		sqlType = "JSONB"
	case "enum":
		// Use VARCHAR for enums, validate at app level
		sqlType = "VARCHAR(50)"
	default:
		sqlType = "TEXT"
	}

	if prop.Required && !prop.Primary {
		constraints = append(constraints, "NOT NULL")
	}

	if prop.Default != "" && prop.Default != "now()" {
		constraints = append(constraints, fmt.Sprintf("DEFAULT '%s'", prop.Default))
	} else if prop.Default == "now()" {
		constraints = append(constraints, "DEFAULT NOW()")
	}

	if len(constraints) > 0 {
		return fmt.Sprintf("%s %s %s", prop.Name, sqlType, strings.Join(constraints, " "))
	}
	return fmt.Sprintf("%s %s", prop.Name, sqlType)
}

// generateIndexDDL generates CREATE INDEX statement
func (m *Manager) generateIndexDDL(idx dsl.Index, table, schema string) string {
	unique := ""
	if idx.Unique {
		unique = "UNIQUE "
	}

	return fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s.%s (%s)",
		unique, idx.Name, schema, table, strings.Join(idx.Fields, ", "))
}

// generateForeignKeyDDL generates ALTER TABLE ADD CONSTRAINT for FK
func (m *Manager) generateForeignKeyDDL(edge dsl.Edge, schema string) string {
	// Find source and target nodes
	sourceNode := m.graph.GetNode(edge.ForeignKey.OnNode)
	targetNode := m.graph.GetNode(edge.To)

	if sourceNode == nil || targetNode == nil {
		return ""
	}

	onDelete := "RESTRICT"
	switch edge.ForeignKey.OnDelete {
	case "cascade":
		onDelete = "CASCADE"
	case "set_null":
		onDelete = "SET NULL"
	}

	return fmt.Sprintf(`
ALTER TABLE %s.%s 
ADD CONSTRAINT fk_%s_%s 
FOREIGN KEY (%s) REFERENCES %s.%s(id) 
ON DELETE %s`,
		schema, sourceNode.Table,
		sourceNode.Table, edge.ForeignKey.Field,
		edge.ForeignKey.Field,
		schema, targetNode.Table,
		onDelete)
}

// DropTenantSchema removes a tenant's schema
func (m *Manager) DropTenantSchema(ctx context.Context, tenantID string) error {
	schemaName := fmt.Sprintf("tenant_%s", tenantID)
	_, err := m.db.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
	return err
}

// MigrateNode handles schema changes for a node (basic implementation)
func (m *Manager) MigrateNode(ctx context.Context, tenantID string, node dsl.Node) error {
	// In production, you'd compare current schema with DSL and generate ALTER statements
	// For now, this is a placeholder
	return nil
}
