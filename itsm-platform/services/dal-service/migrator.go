package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrator handles schema migrations
type Migrator struct {
	db       *pgxpool.Pool
	registry *ServiceRegistry
}

func NewMigrator(db *pgxpool.Pool, registry *ServiceRegistry) *Migrator {
	return &Migrator{
		db:       db,
		registry: registry,
	}
}

// Migrate performs schema migration for a service
func (m *Migrator) Migrate(ctx context.Context, serviceName string, newDSL DSLDefinition) error {
	// Get existing DSL
	existingService := m.registry.GetService(serviceName)

	if existingService == nil {
		// New service - create schemas for all tenants
		return m.createNewService(ctx, serviceName, newDSL)
	}

	// Compare and migrate
	oldDSL := existingService.DSL
	migrations := m.compareDSL(oldDSL, newDSL)

	if len(migrations) == 0 {
		return nil // No changes
	}

	// Get all tenants
	tenants, err := m.listTenants(ctx)
	if err != nil {
		return err
	}

	// Apply migrations to each tenant
	for _, tenant := range tenants {
		schemaName := fmt.Sprintf("tenant_%s", tenant)
		if err := m.applyMigrations(ctx, schemaName, migrations); err != nil {
			return fmt.Errorf("migration failed for tenant %s: %w", tenant, err)
		}
	}

	// Update registry
	return m.registry.RegisterService(serviceName, newDSL)
}

// compareDSL compares old and new DSL to generate migrations
func (m *Migrator) compareDSL(old, new DSLDefinition) []Migration {
	var migrations []Migration

	// Check for new nodes (tables)
	oldNodes := make(map[string]NodeDefinition)
	for _, node := range old.Nodes {
		oldNodes[node.Name] = node
	}

	newNodes := make(map[string]NodeDefinition)
	for _, node := range new.Nodes {
		newNodes[node.Name] = node
	}

	// Find new tables
	for name, node := range newNodes {
		if _, exists := oldNodes[name]; !exists {
			migrations = append(migrations, Migration{
				Type:  "CREATE_TABLE",
				Table: node.Table,
				Node:  &node,
			})
		} else {
			// Compare properties
			oldNode := oldNodes[name]
			tableMigrations := m.compareNodes(oldNode, node)
			migrations = append(migrations, tableMigrations...)
		}
	}

	// Find dropped tables
	for name, node := range oldNodes {
		if _, exists := newNodes[name]; !exists {
			migrations = append(migrations, Migration{
				Type:  "DROP_TABLE",
				Table: node.Table,
			})
		}
	}

	return migrations
}

// compareNodes compares two node definitions
func (m *Migrator) compareNodes(old, new NodeDefinition) []Migration {
	var migrations []Migration

	// Compare properties
	oldProps := make(map[string]PropertyDefinition)
	for _, prop := range old.Properties {
		oldProps[prop.Name] = prop
	}

	newProps := make(map[string]PropertyDefinition)
	for _, prop := range new.Properties {
		newProps[prop.Name] = prop
	}

	// Find new columns
	for name, prop := range newProps {
		if _, exists := oldProps[name]; !exists {
			migrations = append(migrations, Migration{
				Type:     "ADD_COLUMN",
				Table:    new.Table,
				Column:   name,
				Property: &prop,
			})
		} else {
			// Check if type changed
			oldProp := oldProps[name]
			if oldProp.Type != prop.Type {
				migrations = append(migrations, Migration{
					Type:     "ALTER_COLUMN",
					Table:    new.Table,
					Column:   name,
					Property: &prop,
				})
			}
		}
	}

	// Find dropped columns
	for name := range oldProps {
		if _, exists := newProps[name]; !exists {
			migrations = append(migrations, Migration{
				Type:   "DROP_COLUMN",
				Table:  new.Table,
				Column: name,
			})
		}
	}

	// Compare indexes
	migrations = append(migrations, m.compareIndexes(old, new)...)

	return migrations
}

// compareIndexes compares indexes between nodes
func (m *Migrator) compareIndexes(old, new NodeDefinition) []Migration {
	var migrations []Migration

	oldIndexes := make(map[string]IndexDefinition)
	for _, idx := range old.Indexes {
		oldIndexes[idx.Name] = idx
	}

	newIndexes := make(map[string]IndexDefinition)
	for _, idx := range new.Indexes {
		newIndexes[idx.Name] = idx
	}

	// Find new indexes
	for name, idx := range newIndexes {
		if _, exists := oldIndexes[name]; !exists {
			migrations = append(migrations, Migration{
				Type:  "CREATE_INDEX",
				Table: new.Table,
				Index: &idx,
			})
		}
	}

	// Find dropped indexes
	for name := range oldIndexes {
		if _, exists := newIndexes[name]; !exists {
			migrations = append(migrations, Migration{
				Type:      "DROP_INDEX",
				Table:     new.Table,
				IndexName: name,
			})
		}
	}

	return migrations
}

// applyMigrations applies migrations to a schema
func (m *Migrator) applyMigrations(ctx context.Context, schema string, migrations []Migration) error {
	for _, migration := range migrations {
		sql := m.generateSQL(schema, migration)
		if sql == "" {
			continue
		}

		if _, err := m.db.Exec(ctx, sql); err != nil {
			return fmt.Errorf("migration failed: %s - %w", sql, err)
		}
	}
	return nil
}

// generateSQL generates SQL for a migration
func (m *Migrator) generateSQL(schema string, migration Migration) string {
	tableName := fmt.Sprintf("%s.%s", schema, migration.Table)

	switch migration.Type {
	case "CREATE_TABLE":
		return m.generateCreateTable(schema, migration.Node)

	case "DROP_TABLE":
		return fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", tableName)

	case "ADD_COLUMN":
		colDef := m.buildColumnDef(migration.Property)
		return fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s", tableName, colDef)

	case "DROP_COLUMN":
		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s", tableName, migration.Column)

	case "ALTER_COLUMN":
		// This is simplified - real implementation would handle type conversions
		return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s",
			tableName, migration.Column, m.mapType(migration.Property.Type))

	case "CREATE_INDEX":
		idxName := fmt.Sprintf("%s_%s", migration.Table, migration.Index.Name)
		unique := ""
		if migration.Index.Unique {
			unique = "UNIQUE "
		}
		fields := strings.Join(migration.Index.Fields, ", ")
		return fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s(%s)",
			unique, idxName, tableName, fields)

	case "DROP_INDEX":
		idxName := fmt.Sprintf("%s_%s", migration.Table, migration.IndexName)
		return fmt.Sprintf("DROP INDEX IF EXISTS %s.%s", schema, idxName)

	default:
		return ""
	}
}

// generateCreateTable generates CREATE TABLE SQL
func (m *Migrator) generateCreateTable(schema string, node *NodeDefinition) string {
	sm := NewSchemaManager(m.db)
	// Reuse SchemaManager's createTable logic
	ctx := context.Background()
	sm.createTable(ctx, schema, *node)
	return "" // Already executed
}

// buildColumnDef builds column definition SQL
func (m *Migrator) buildColumnDef(prop *PropertyDefinition) string {
	var col strings.Builder
	col.WriteString(prop.Name)
	col.WriteString(" ")
	col.WriteString(m.mapType(prop.Type))

	if prop.Required {
		col.WriteString(" NOT NULL")
	}

	if prop.Default != "" {
		col.WriteString(fmt.Sprintf(" DEFAULT %s", prop.Default))
	}

	return col.String()
}

// mapType maps DSL type to SQL type
func (m *Migrator) mapType(dslType string) string {
	switch dslType {
	case "string":
		return "TEXT"
	case "int", "integer":
		return "INTEGER"
	case "bigint":
		return "BIGINT"
	case "boolean", "bool":
		return "BOOLEAN"
	case "uuid":
		return "UUID"
	case "datetime", "timestamp":
		return "TIMESTAMPTZ"
	case "json", "jsonb":
		return "JSONB"
	default:
		return "TEXT"
	}
}

// createNewService creates schema for a new service
func (m *Migrator) createNewService(ctx context.Context, serviceName string, dsl DSLDefinition) error {
	// Register service
	if err := m.registry.RegisterService(serviceName, dsl); err != nil {
		return err
	}

	// Get all tenants
	tenants, err := m.listTenants(ctx)
	if err != nil {
		return err
	}

	// Create schemas for all tenants
	sm := NewSchemaManager(m.db)
	for _, tenant := range tenants {
		if err := sm.CreateServiceSchema(ctx, tenant, serviceName, dsl); err != nil {
			return fmt.Errorf("failed to create schema for tenant %s: %w", tenant, err)
		}
	}

	return nil
}

// listTenants lists all tenant IDs
func (m *Migrator) listTenants(ctx context.Context) ([]string, error) {
	sm := NewSchemaManager(m.db)
	return sm.ListTenants(ctx)
}

// Migration represents a schema change
type Migration struct {
	Type      string
	Table     string
	Column    string
	Property  *PropertyDefinition
	Node      *NodeDefinition
	Index     *IndexDefinition
	IndexName string
}
