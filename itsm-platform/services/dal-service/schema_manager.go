package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SchemaManager struct {
	db *pgxpool.Pool
}

func NewSchemaManager(db *pgxpool.Pool) *SchemaManager {
	return &SchemaManager{db: db}
}

// CreateTenantSchema creates a new schema for a tenant
func (sm *SchemaManager) CreateTenantSchema(ctx context.Context, tenantID string) error {
	schemaName := fmt.Sprintf("tenant_%s", tenantID)

	// Create schema if not exists
	query := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schemaName)
	_, err := sm.db.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Create audit table for tenant
	auditTable := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.audit_log (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			entity_type VARCHAR(100),
			entity_id UUID,
			action VARCHAR(50),
			changes JSONB,
			user_id UUID,
			timestamp TIMESTAMPTZ DEFAULT NOW(),
			metadata JSONB
		)`, schemaName)

	_, err = sm.db.Exec(ctx, auditTable)
	return err
}

// CreateServiceSchema creates tables for a service in tenant schema
func (sm *SchemaManager) CreateServiceSchema(ctx context.Context, tenantID, service string, dsl DSLDefinition) error {
	schemaName := fmt.Sprintf("tenant_%s", tenantID)

	for _, node := range dsl.Nodes {
		if err := sm.createTable(ctx, schemaName, node); err != nil {
			return fmt.Errorf("failed to create table %s: %w", node.Table, err)
		}

		if err := sm.createIndexes(ctx, schemaName, node); err != nil {
			return fmt.Errorf("failed to create indexes for %s: %w", node.Table, err)
		}
	}

	return nil
}

func (sm *SchemaManager) createTable(ctx context.Context, schema string, node NodeDefinition) error {
	var columns []string
	existingCols := make(map[string]bool)

	// Add entity properties first (from DSL)
	for _, prop := range node.Properties {
		col := sm.buildColumnDefinition(prop)
		columns = append(columns, col)
		existingCols[prop.Name] = true
	}

	// Add system columns if not already defined in DSL
	systemCols := map[string]string{
		"created_by": "UUID",
		"updated_by": "UUID",
	}

	for colName, colDef := range systemCols {
		if !existingCols[colName] {
			columns = append(columns, fmt.Sprintf("%s %s", colName, colDef))
		}
	}

	// Add soft delete if configured
	if node.DAL.SoftDelete {
		if !existingCols["deleted_at"] {
			columns = append(columns, "deleted_at TIMESTAMPTZ")
		}
		if !existingCols["deleted_by"] {
			columns = append(columns, "deleted_by UUID")
		}
	}

	// Add optimistic lock if configured
	if node.DAL.OptimisticLock {
		if !existingCols["version"] {
			columns = append(columns, "version INTEGER DEFAULT 1")
		}
	}

	// Build CREATE TABLE statement
	tableName := fmt.Sprintf("%s.%s", schema, node.Table)
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n)",
		tableName, strings.Join(columns, ",\n"))

	_, err := sm.db.Exec(ctx, query)
	return err
}

func (sm *SchemaManager) buildColumnDefinition(prop PropertyDefinition) string {
	var col strings.Builder
	col.WriteString(prop.Name)
	col.WriteString(" ")

	// Map DSL type to PostgreSQL type
	switch prop.Type {
	case "string":
		if prop.MaxLength > 0 {
			col.WriteString(fmt.Sprintf("VARCHAR(%d)", prop.MaxLength))
		} else {
			col.WriteString("TEXT")
		}
	case "int", "integer":
		col.WriteString("INTEGER")
	case "bigint":
		col.WriteString("BIGINT")
	case "decimal":
		col.WriteString(fmt.Sprintf("DECIMAL(%d,%d)", prop.Precision, prop.Scale))
	case "boolean", "bool":
		col.WriteString("BOOLEAN")
	case "uuid":
		col.WriteString("UUID")
	case "date":
		col.WriteString("DATE")
	case "datetime", "timestamp":
		col.WriteString("TIMESTAMPTZ")
	case "json", "jsonb":
		col.WriteString("JSONB")
	case "enum":
		// For enums, use VARCHAR with CHECK constraint
		col.WriteString("VARCHAR(50)")
		if len(prop.Values) > 0 {
			values := make([]string, len(prop.Values))
			for i, v := range prop.Values {
				values[i] = fmt.Sprintf("'%s'", v)
			}
			col.WriteString(fmt.Sprintf(" CHECK (%s IN (%s))",
				prop.Name, strings.Join(values, ", ")))
		}
	case "text":
		col.WriteString("TEXT")
	case "array":
		col.WriteString("JSONB") // Store arrays as JSONB
	default:
		col.WriteString("TEXT")
	}

	// Add constraints
	if prop.Primary {
		col.WriteString(" PRIMARY KEY")
		if prop.Type == "uuid" {
			col.WriteString(" DEFAULT gen_random_uuid()")
		}
	} else if prop.Required {
		col.WriteString(" NOT NULL")
	}

	if prop.Default != nil {
		// Handle different default value types
		switch v := prop.Default.(type) {
		case string:
			if v == "now()" {
				col.WriteString(" DEFAULT NOW()")
			} else if v != "" {
				col.WriteString(fmt.Sprintf(" DEFAULT '%s'", v))
			}
		case bool:
			col.WriteString(fmt.Sprintf(" DEFAULT %t", v))
		case int, int64, float64:
			col.WriteString(fmt.Sprintf(" DEFAULT %v", v))
		}
	}

	if prop.UniquePerTenant {
		// Unique constraint will be added as index
	}

	return col.String()
}

func (sm *SchemaManager) createIndexes(ctx context.Context, schema string, node NodeDefinition) error {
	tableName := fmt.Sprintf("%s.%s", schema, node.Table)

	// Create index on tenant_id (always)
	tenantIdx := fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_tenant_id ON %s(tenant_id)",
		node.Table, tableName)
	if _, err := sm.db.Exec(ctx, tenantIdx); err != nil {
		return err
	}

	// Create indexes for properties marked as indexed
	for _, prop := range node.Properties {
		// Check if property should be indexed (can be bool or string)
		shouldIndex := false
		if prop.Indexed != nil {
			switch v := prop.Indexed.(type) {
			case bool:
				shouldIndex = v
			case string:
				shouldIndex = v != ""
			}
		}

		if shouldIndex {
			idxName := fmt.Sprintf("idx_%s_%s", node.Table, prop.Name)
			var indexType string
			if indexTypeStr, ok := prop.Indexed.(string); ok && indexTypeStr != "" {
				// Use specific index type if provided
				indexType = fmt.Sprintf(" USING %s", indexTypeStr)
			}
			query := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s%s(%s)",
				idxName, tableName, indexType, prop.Name)
			if _, err := sm.db.Exec(ctx, query); err != nil {
				return err
			}
		}

		// Create unique index for unique_per_tenant fields
		if prop.UniquePerTenant {
			idxName := fmt.Sprintf("uniq_%s_%s_tenant", node.Table, prop.Name)
			query := fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s(tenant_id, %s)",
				idxName, tableName, prop.Name)
			if _, err := sm.db.Exec(ctx, query); err != nil {
				return err
			}
		}
	}

	// Create custom indexes
	for _, idx := range node.Indexes {
		idxName := fmt.Sprintf("%s_%s", node.Table, idx.Name)
		fields := strings.Join(idx.Fields, ", ")
		unique := ""
		if idx.Unique {
			unique = "UNIQUE "
		}
		query := fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s(%s)",
			unique, idxName, tableName, fields)
		if _, err := sm.db.Exec(ctx, query); err != nil {
			return err
		}
	}

	// Create soft delete partial index
	if node.DAL.SoftDelete {
		idxName := fmt.Sprintf("idx_%s_not_deleted", node.Table)
		query := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s(tenant_id) WHERE deleted_at IS NULL",
			idxName, tableName)
		if _, err := sm.db.Exec(ctx, query); err != nil {
			return err
		}
	}

	return nil
}

// ListTenants returns all tenant IDs
func (sm *SchemaManager) ListTenants(ctx context.Context) ([]string, error) {
	query := `
		SELECT schema_name 
		FROM information_schema.schemata 
		WHERE schema_name LIKE 'tenant_%'
	`

	rows, err := sm.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []string
	for rows.Next() {
		var schemaName string
		if err := rows.Scan(&schemaName); err != nil {
			continue
		}
		// Extract tenant ID from schema name
		tenantID := strings.TrimPrefix(schemaName, "tenant_")
		tenants = append(tenants, tenantID)
	}

	return tenants, nil
}

// DropTenantSchema removes a tenant's schema (careful!)
func (sm *SchemaManager) DropTenantSchema(ctx context.Context, tenantID string) error {
	schemaName := fmt.Sprintf("tenant_%s", tenantID)
	query := fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName)
	_, err := sm.db.Exec(ctx, query)
	return err
}
