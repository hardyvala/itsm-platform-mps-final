package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QueryExecutor struct {
	db      *pgxpool.Pool
	service *ServiceDefinition
}

func NewQueryExecutor(db *pgxpool.Pool, service *ServiceDefinition) *QueryExecutor {
	return &QueryExecutor{
		db:      db,
		service: service,
	}
}

// Execute runs a query based on DSL Query format
func (qe *QueryExecutor) Execute(ctx context.Context, tenantID, entityName string, query Query) ([]map[string]interface{}, int64, error) {
	node := qe.service.GetNode(entityName)
	if node == nil {
		return nil, 0, fmt.Errorf("entity %s not found", entityName)
	}

	schemaName := fmt.Sprintf("tenant_%s", tenantID)
	tableName := fmt.Sprintf("%s.%s", schemaName, node.Table)

	// Build SELECT clause
	selectClause := qe.buildSelectClause(query.Select, node)

	// Build WHERE clause
	whereClause, whereParams := qe.buildWhereClause(query.Where, tenantID, node)

	// Build ORDER BY
	orderClause := qe.buildOrderClause(query.OrderBy)

	// Build query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", tableName, whereClause)

	mainQuery := fmt.Sprintf("SELECT %s FROM %s WHERE %s",
		selectClause, tableName, whereClause)

	if orderClause != "" {
		mainQuery += " ORDER BY " + orderClause
	}

	// Add pagination
	if query.Limit > 0 {
		mainQuery += fmt.Sprintf(" LIMIT %d", query.Limit)
	}
	if query.Offset > 0 {
		mainQuery += fmt.Sprintf(" OFFSET %d", query.Offset)
	}

	// Execute count query
	var total int64
	err := qe.db.QueryRow(ctx, countQuery, whereParams...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count query failed: %w", err)
	}

	// Execute main query
	rows, err := qe.db.Query(ctx, mainQuery, whereParams...)
	if err != nil {
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	results, err := qe.scanRows(rows)
	if err != nil {
		return nil, 0, err
	}

	// Handle relations if requested
	if len(query.Relations) > 0 {
		results = qe.fetchRelations(ctx, tenantID, results, query.Relations)
	}

	return results, total, nil
}

// Create inserts a new record
func (qe *QueryExecutor) Create(ctx context.Context, tenantID, entityName string, data map[string]interface{}) (map[string]interface{}, error) {
	node := qe.service.GetNode(entityName)
	if node == nil {
		return nil, fmt.Errorf("entity %s not found", entityName)
	}

	schemaName := fmt.Sprintf("tenant_%s", tenantID)
	tableName := fmt.Sprintf("%s.%s", schemaName, node.Table)

	// Add system fields
	data["id"] = uuid.New().String()
	data["tenant_id"] = tenantID
	data["created_at"] = time.Now().UTC()
	data["updated_at"] = time.Now().UTC()

	if node.DAL.OptimisticLock {
		data["version"] = 1
	}

	// Build INSERT query
	columns := make([]string, 0, len(data))
	placeholders := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data))

	i := 1
	for col, val := range data {
		columns = append(columns, col)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		values = append(values, val)
		i++
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING *",
		tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	row := qe.db.QueryRow(ctx, query, values...)
	result, err := qe.scanRow(row, node)
	if err != nil {
		return nil, fmt.Errorf("insert failed: %w", err)
	}

	return result, nil
}

// Update modifies an existing record
func (qe *QueryExecutor) Update(ctx context.Context, tenantID, entityName, id string, data map[string]interface{}) (map[string]interface{}, error) {
	node := qe.service.GetNode(entityName)
	if node == nil {
		return nil, fmt.Errorf("entity %s not found", entityName)
	}

	schemaName := fmt.Sprintf("tenant_%s", tenantID)
	tableName := fmt.Sprintf("%s.%s", schemaName, node.Table)

	// Get current version if using optimistic locking
	var currentVersion int
	if node.DAL.OptimisticLock {
		vQuery := fmt.Sprintf("SELECT version FROM %s WHERE id = $1 AND tenant_id = $2", tableName)
		err := qe.db.QueryRow(ctx, vQuery, id, tenantID).Scan(&currentVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to get current version: %w", err)
		}
		data["version"] = currentVersion + 1
	}

	// Add updated_at
	data["updated_at"] = time.Now().UTC()

	// Build UPDATE query
	setClauses := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data)+2)

	i := 1
	for col, val := range data {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		values = append(values, val)
		i++
	}

	values = append(values, id, tenantID)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d AND tenant_id = $%d",
		tableName, strings.Join(setClauses, ", "), i, i+1)

	// Add optimistic lock check
	if node.DAL.OptimisticLock {
		query += fmt.Sprintf(" AND version = %d", currentVersion)
	}

	query += " RETURNING *"

	row := qe.db.QueryRow(ctx, query, values...)
	result, err := qe.scanRow(row, node)
	if err != nil {
		if err == pgx.ErrNoRows && node.DAL.OptimisticLock {
			return nil, fmt.Errorf("optimistic lock conflict")
		}
		return nil, fmt.Errorf("update failed: %w", err)
	}

	return result, nil
}

// Delete removes a record (soft or hard delete based on DSL)
func (qe *QueryExecutor) Delete(ctx context.Context, tenantID, entityName, id string) error {
	node := qe.service.GetNode(entityName)
	if node == nil {
		return fmt.Errorf("entity %s not found", entityName)
	}

	schemaName := fmt.Sprintf("tenant_%s", tenantID)
	tableName := fmt.Sprintf("%s.%s", schemaName, node.Table)

	var query string
	var params []interface{}

	if node.DAL.SoftDelete {
		// Soft delete
		query = fmt.Sprintf("UPDATE %s SET deleted_at = $1 WHERE id = $2 AND tenant_id = $3",
			tableName)
		params = []interface{}{time.Now().UTC(), id, tenantID}
	} else {
		// Hard delete
		query = fmt.Sprintf("DELETE FROM %s WHERE id = $1 AND tenant_id = $2", tableName)
		params = []interface{}{id, tenantID}
	}

	result, err := qe.db.Exec(ctx, query, params...)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("entity not found")
	}

	return nil
}

// GetByID retrieves a single record
func (qe *QueryExecutor) GetByID(ctx context.Context, tenantID, entityName, id string) (map[string]interface{}, error) {
	node := qe.service.GetNode(entityName)
	if node == nil {
		return nil, fmt.Errorf("entity %s not found", entityName)
	}

	schemaName := fmt.Sprintf("tenant_%s", tenantID)
	tableName := fmt.Sprintf("%s.%s", schemaName, node.Table)

	query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1 AND tenant_id = $2", tableName)

	if node.DAL.SoftDelete {
		query += " AND deleted_at IS NULL"
	}

	row := qe.db.QueryRow(ctx, query, id, tenantID)
	return qe.scanRow(row, node)
}

func (qe *QueryExecutor) buildSelectClause(fields []string, node *NodeDefinition) string {
	if len(fields) == 0 {
		return "*"
	}

	// Validate fields exist
	validFields := make([]string, 0, len(fields))
	for _, field := range fields {
		if field == "*" {
			return "*"
		}
		// Check if field exists in node properties or is a system field
		if qe.isValidField(field, node) {
			validFields = append(validFields, field)
		}
	}

	if len(validFields) == 0 {
		return "*"
	}

	return strings.Join(validFields, ", ")
}

func (qe *QueryExecutor) buildWhereClause(conditions []Condition, tenantID string, node *NodeDefinition) (string, []interface{}) {
	clauses := []string{"tenant_id = $1"}
	params := []interface{}{tenantID}
	paramCount := 1

	// Add soft delete filter
	if node.DAL.SoftDelete {
		clauses = append(clauses, "deleted_at IS NULL")
	}

	for _, cond := range conditions {
		paramCount++
		clause, param := qe.buildCondition(cond, paramCount)
		if clause != "" {
			clauses = append(clauses, clause)
			if param != nil {
				params = append(params, param)
			}
		}
	}

	return strings.Join(clauses, " AND "), params
}

func (qe *QueryExecutor) buildCondition(cond Condition, paramNum int) (string, interface{}) {
	switch cond.Operator {
	case "eq", "=":
		return fmt.Sprintf("%s = $%d", cond.Field, paramNum), cond.Value
	case "ne", "!=":
		return fmt.Sprintf("%s != $%d", cond.Field, paramNum), cond.Value
	case "gt", ">":
		return fmt.Sprintf("%s > $%d", cond.Field, paramNum), cond.Value
	case "gte", ">=":
		return fmt.Sprintf("%s >= $%d", cond.Field, paramNum), cond.Value
	case "lt", "<":
		return fmt.Sprintf("%s < $%d", cond.Field, paramNum), cond.Value
	case "lte", "<=":
		return fmt.Sprintf("%s <= $%d", cond.Field, paramNum), cond.Value
	case "like":
		return fmt.Sprintf("%s LIKE $%d", cond.Field, paramNum), cond.Value
	case "in":
		// Handle array values
		if arr, ok := cond.Value.([]interface{}); ok {
			placeholders := make([]string, len(arr))
			for i := range arr {
				placeholders[i] = fmt.Sprintf("$%d", paramNum+i)
			}
			return fmt.Sprintf("%s IN (%s)", cond.Field, strings.Join(placeholders, ",")), arr
		}
		return "", nil
	case "is_null":
		return fmt.Sprintf("%s IS NULL", cond.Field), nil
	case "is_not_null":
		return fmt.Sprintf("%s IS NOT NULL", cond.Field), nil
	default:
		return "", nil
	}
}

func (qe *QueryExecutor) buildOrderClause(orderBy []OrderBy) string {
	if len(orderBy) == 0 {
		return ""
	}

	clauses := make([]string, len(orderBy))
	for i, order := range orderBy {
		direction := "ASC"
		if order.Desc {
			direction = "DESC"
		}
		clauses[i] = fmt.Sprintf("%s %s", order.Field, direction)
	}

	return strings.Join(clauses, ", ")
}

func (qe *QueryExecutor) scanRow(row pgx.Row, node *NodeDefinition) (map[string]interface{}, error) {
	fieldDescriptions := []string{
		"id", "tenant_id", "created_at", "updated_at", "created_by", "updated_by",
	}

	if node.DAL.SoftDelete {
		fieldDescriptions = append(fieldDescriptions, "deleted_at", "deleted_by")
	}
	if node.DAL.OptimisticLock {
		fieldDescriptions = append(fieldDescriptions, "version")
	}

	for _, prop := range node.Properties {
		fieldDescriptions = append(fieldDescriptions, prop.Name)
	}

	values := make([]interface{}, len(fieldDescriptions))
	valuePtrs := make([]interface{}, len(fieldDescriptions))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := row.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	for i, field := range fieldDescriptions {
		result[field] = values[i]
	}

	return result, nil
}

func (qe *QueryExecutor) scanRows(rows pgx.Rows) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}

		fields := rows.FieldDescriptions()
		result := make(map[string]interface{})

		for i, field := range fields {
			result[string(field.Name)] = values[i]
		}

		results = append(results, result)
	}

	return results, rows.Err()
}

func (qe *QueryExecutor) isValidField(field string, node *NodeDefinition) bool {
	systemFields := []string{
		"id", "tenant_id", "created_at", "updated_at",
		"created_by", "updated_by", "deleted_at", "deleted_by", "version",
	}

	for _, f := range systemFields {
		if f == field {
			return true
		}
	}

	for _, prop := range node.Properties {
		if prop.Name == field {
			return true
		}
	}

	return false
}

func (qe *QueryExecutor) fetchRelations(ctx context.Context, tenantID string, results []map[string]interface{}, relations []RelationQuery) []map[string]interface{} {
	// This would handle fetching related data
	// For cross-service relations, it would make NATS calls
	// For same-service, direct DB queries
	return results
}
