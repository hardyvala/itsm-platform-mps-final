package query

import (
	"fmt"
	"strings"
)

// Query represents a JSON query from UI
// UI sends this structure, SDK converts to SQL
type Query struct {
	Select    []string               `json:"select,omitempty"`    // Fields to select, empty = all
	From      string                 `json:"from"`                // Table/entity name
	Where     []Condition            `json:"where,omitempty"`     // Filter conditions
	OrderBy   []OrderClause          `json:"order_by,omitempty"`  // Sorting
	Limit     int                    `json:"limit,omitempty"`     // Pagination
	Offset    int                    `json:"offset,omitempty"`    // Pagination
	Relations []RelationQuery        `json:"relations,omitempty"` // Related data to fetch
}

// Condition represents a WHERE condition
type Condition struct {
	Field    string      `json:"field"`
	Operator string      `json:"op"`    // eq, neq, gt, gte, lt, lte, like, ilike, in, is_null, is_not_null
	Value    interface{} `json:"value"`
	Or       []Condition `json:"or,omitempty"`  // OR conditions
	And      []Condition `json:"and,omitempty"` // AND conditions
}

// OrderClause represents ORDER BY
type OrderClause struct {
	Field string `json:"field"`
	Dir   string `json:"dir"` // asc, desc
}

// RelationQuery for fetching related data (handled separately, not JOIN)
type RelationQuery struct {
	Name   string   `json:"name"`             // Relation name from DSL
	Select []string `json:"select,omitempty"` // Fields from related entity
}

// Builder converts Query to SQL
type Builder struct {
	schema     string // tenant schema
	properties map[string]bool // valid properties from DSL
}

// NewBuilder creates a query builder
func NewBuilder(schema string, validProperties []string) *Builder {
	props := make(map[string]bool)
	for _, p := range validProperties {
		props[p] = true
	}
	return &Builder{schema: schema, properties: props}
}

// Build converts Query to SQL and parameters
func (b *Builder) Build(q Query) (string, []interface{}, error) {
	var sql strings.Builder
	var params []interface{}
	paramIdx := 1

	// SELECT
	if len(q.Select) == 0 {
		sql.WriteString("SELECT *")
	} else {
		// Validate fields
		for _, f := range q.Select {
			if !b.properties[f] {
				return "", nil, fmt.Errorf("invalid field: %s", f)
			}
		}
		sql.WriteString("SELECT ")
		sql.WriteString(strings.Join(q.Select, ", "))
	}

	// FROM
	sql.WriteString(fmt.Sprintf(" FROM %s.%s", b.schema, q.From))

	// WHERE
	if len(q.Where) > 0 {
		whereSQL, whereParams, idx, err := b.buildConditions(q.Where, paramIdx)
		if err != nil {
			return "", nil, err
		}
		sql.WriteString(" WHERE ")
		sql.WriteString(whereSQL)
		params = append(params, whereParams...)
		paramIdx = idx
	}

	// ORDER BY
	if len(q.OrderBy) > 0 {
		sql.WriteString(" ORDER BY ")
		var orders []string
		for _, o := range q.OrderBy {
			if !b.properties[o.Field] {
				return "", nil, fmt.Errorf("invalid order field: %s", o.Field)
			}
			dir := "ASC"
			if strings.ToUpper(o.Dir) == "DESC" {
				dir = "DESC"
			}
			orders = append(orders, fmt.Sprintf("%s %s", o.Field, dir))
		}
		sql.WriteString(strings.Join(orders, ", "))
	}

	// LIMIT & OFFSET
	if q.Limit > 0 {
		sql.WriteString(fmt.Sprintf(" LIMIT %d", q.Limit))
	}
	if q.Offset > 0 {
		sql.WriteString(fmt.Sprintf(" OFFSET %d", q.Offset))
	}

	return sql.String(), params, nil
}

// buildConditions recursively builds WHERE conditions
func (b *Builder) buildConditions(conditions []Condition, paramIdx int) (string, []interface{}, int, error) {
	var parts []string
	var params []interface{}

	for _, c := range conditions {
		// Handle OR groups
		if len(c.Or) > 0 {
			orSQL, orParams, idx, err := b.buildConditions(c.Or, paramIdx)
			if err != nil {
				return "", nil, 0, err
			}
			parts = append(parts, fmt.Sprintf("(%s)", strings.ReplaceAll(orSQL, " AND ", " OR ")))
			params = append(params, orParams...)
			paramIdx = idx
			continue
		}

		// Handle AND groups
		if len(c.And) > 0 {
			andSQL, andParams, idx, err := b.buildConditions(c.And, paramIdx)
			if err != nil {
				return "", nil, 0, err
			}
			parts = append(parts, fmt.Sprintf("(%s)", andSQL))
			params = append(params, andParams...)
			paramIdx = idx
			continue
		}

		// Validate field
		if !b.properties[c.Field] {
			return "", nil, 0, fmt.Errorf("invalid field: %s", c.Field)
		}

		// Build condition based on operator
		condSQL, condParams, idx := b.buildSingleCondition(c, paramIdx)
		parts = append(parts, condSQL)
		params = append(params, condParams...)
		paramIdx = idx
	}

	return strings.Join(parts, " AND "), params, paramIdx, nil
}

// buildSingleCondition builds a single condition
func (b *Builder) buildSingleCondition(c Condition, paramIdx int) (string, []interface{}, int) {
	var sql string
	var params []interface{}

	switch c.Operator {
	case "eq", "=", "":
		sql = fmt.Sprintf("%s = $%d", c.Field, paramIdx)
		params = append(params, c.Value)
		paramIdx++

	case "neq", "!=", "<>":
		sql = fmt.Sprintf("%s != $%d", c.Field, paramIdx)
		params = append(params, c.Value)
		paramIdx++

	case "gt", ">":
		sql = fmt.Sprintf("%s > $%d", c.Field, paramIdx)
		params = append(params, c.Value)
		paramIdx++

	case "gte", ">=":
		sql = fmt.Sprintf("%s >= $%d", c.Field, paramIdx)
		params = append(params, c.Value)
		paramIdx++

	case "lt", "<":
		sql = fmt.Sprintf("%s < $%d", c.Field, paramIdx)
		params = append(params, c.Value)
		paramIdx++

	case "lte", "<=":
		sql = fmt.Sprintf("%s <= $%d", c.Field, paramIdx)
		params = append(params, c.Value)
		paramIdx++

	case "like":
		sql = fmt.Sprintf("%s LIKE $%d", c.Field, paramIdx)
		params = append(params, c.Value)
		paramIdx++

	case "ilike":
		sql = fmt.Sprintf("%s ILIKE $%d", c.Field, paramIdx)
		params = append(params, c.Value)
		paramIdx++

	case "in":
		if values, ok := c.Value.([]interface{}); ok {
			placeholders := make([]string, len(values))
			for i, v := range values {
				placeholders[i] = fmt.Sprintf("$%d", paramIdx)
				params = append(params, v)
				paramIdx++
			}
			sql = fmt.Sprintf("%s IN (%s)", c.Field, strings.Join(placeholders, ", "))
		}

	case "not_in":
		if values, ok := c.Value.([]interface{}); ok {
			placeholders := make([]string, len(values))
			for i, v := range values {
				placeholders[i] = fmt.Sprintf("$%d", paramIdx)
				params = append(params, v)
				paramIdx++
			}
			sql = fmt.Sprintf("%s NOT IN (%s)", c.Field, strings.Join(placeholders, ", "))
		}

	case "is_null":
		sql = fmt.Sprintf("%s IS NULL", c.Field)

	case "is_not_null":
		sql = fmt.Sprintf("%s IS NOT NULL", c.Field)

	case "between":
		if values, ok := c.Value.([]interface{}); ok && len(values) == 2 {
			sql = fmt.Sprintf("%s BETWEEN $%d AND $%d", c.Field, paramIdx, paramIdx+1)
			params = append(params, values[0], values[1])
			paramIdx += 2
		}

	default:
		sql = fmt.Sprintf("%s = $%d", c.Field, paramIdx)
		params = append(params, c.Value)
		paramIdx++
	}

	return sql, params, paramIdx
}

// BuildInsert generates INSERT SQL
func (b *Builder) BuildInsert(table string, data map[string]interface{}) (string, []interface{}, error) {
	var columns []string
	var placeholders []string
	var params []interface{}
	idx := 1

	for col, val := range data {
		if !b.properties[col] {
			continue // Skip unknown fields
		}
		columns = append(columns, col)
		placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
		params = append(params, val)
		idx++
	}

	sql := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s) RETURNING *",
		b.schema, table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	return sql, params, nil
}

// BuildUpdate generates UPDATE SQL
func (b *Builder) BuildUpdate(table string, id string, data map[string]interface{}) (string, []interface{}, error) {
	var setClauses []string
	var params []interface{}
	idx := 1

	for col, val := range data {
		if col == "id" || col == "tenant_id" || col == "created_at" {
			continue // Skip immutable fields
		}
		if !b.properties[col] {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, idx))
		params = append(params, val)
		idx++
	}

	params = append(params, id)
	sql := fmt.Sprintf("UPDATE %s.%s SET %s WHERE id = $%d RETURNING *",
		b.schema, table,
		strings.Join(setClauses, ", "),
		idx)

	return sql, params, nil
}

// BuildDelete generates DELETE (or soft delete) SQL
func (b *Builder) BuildDelete(table string, id string, softDelete bool) (string, []interface{}) {
	if softDelete {
		return fmt.Sprintf("UPDATE %s.%s SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL RETURNING *",
			b.schema, table), []interface{}{id}
	}
	return fmt.Sprintf("DELETE FROM %s.%s WHERE id = $1", b.schema, table), []interface{}{id}
}

// Example UI JSON queries:
//
// Simple query:
// {
//   "from": "tickets",
//   "where": [
//     {"field": "status", "op": "eq", "value": "open"},
//     {"field": "priority", "op": "in", "value": ["high", "critical"]}
//   ],
//   "order_by": [{"field": "created_at", "dir": "desc"}],
//   "limit": 20
// }
//
// Complex query with OR:
// {
//   "from": "tickets",
//   "where": [
//     {"field": "tenant_id", "op": "eq", "value": "xxx"},
//     {
//       "or": [
//         {"field": "status", "op": "eq", "value": "open"},
//         {"field": "priority", "op": "eq", "value": "critical"}
//       ]
//     }
//   ]
// }
//
// Query with relations (fetched separately):
// {
//   "from": "tickets",
//   "where": [{"field": "id", "op": "eq", "value": "xxx"}],
//   "relations": [
//     {"name": "customer", "select": ["id", "name", "email"]},
//     {"name": "comments", "select": ["id", "body", "created_at"]}
//   ]
// }
