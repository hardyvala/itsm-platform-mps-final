package dalclient

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// Client provides access to the DAL service via NATS
type Client struct {
	nc      *nats.Conn
	service string
	timeout time.Duration
}

// NewClient creates a new DAL client
func NewClient(nc *nats.Conn, service string) *Client {
	return &Client{
		nc:      nc,
		service: service,
		timeout: 5 * time.Second,
	}
}

// Query executes a query
func (c *Client) Query(ctx context.Context, tenantID, entity string, query interface{}) (*QueryResult, error) {
	subject := fmt.Sprintf("dal.%s.%s.query", c.service, entity)

	request := map[string]interface{}{
		"tenant_id": tenantID,
		"query":     query,
	}

	return c.request(subject, request)
}

// Create creates a new entity
func (c *Client) Create(ctx context.Context, tenantID, entity string, data map[string]interface{}) (map[string]interface{}, error) {
	subject := fmt.Sprintf("dal.%s.%s.create", c.service, entity)

	request := map[string]interface{}{
		"tenant_id": tenantID,
		"data":      data,
	}

	result, err := c.request(subject, request)
	if err != nil {
		return nil, err
	}

	return result.Data.(map[string]interface{}), nil
}

// Update updates an entity
func (c *Client) Update(ctx context.Context, tenantID, entity, id string, data map[string]interface{}) (map[string]interface{}, error) {
	subject := fmt.Sprintf("dal.%s.%s.update", c.service, entity)

	request := map[string]interface{}{
		"tenant_id": tenantID,
		"id":        id,
		"data":      data,
	}

	result, err := c.request(subject, request)
	if err != nil {
		return nil, err
	}

	return result.Data.(map[string]interface{}), nil
}

// Delete deletes an entity
func (c *Client) Delete(ctx context.Context, tenantID, entity, id string) error {
	subject := fmt.Sprintf("dal.%s.%s.delete", c.service, entity)

	request := map[string]interface{}{
		"tenant_id": tenantID,
		"id":        id,
	}

	_, err := c.request(subject, request)
	return err
}

// Get retrieves an entity by ID
func (c *Client) Get(ctx context.Context, tenantID, entity, id string) (map[string]interface{}, error) {
	subject := fmt.Sprintf("dal.%s.%s.get", c.service, entity)

	request := map[string]interface{}{
		"tenant_id": tenantID,
		"id":        id,
	}

	result, err := c.request(subject, request)
	if err != nil {
		return nil, err
	}

	return result.Data.(map[string]interface{}), nil
}

// RegisterService registers a service DSL with the DAL
func (c *Client) RegisterService(serviceName string, dsl interface{}) error {
	subject := "dal.register"

	request := map[string]interface{}{
		"service": serviceName,
		"dsl":     dsl,
	}

	_, err := c.request(subject, request)
	return err
}

// CreateTenant creates a new tenant schema
func (c *Client) CreateTenant(tenantID string) error {
	subject := "dal.tenant.create"

	request := map[string]interface{}{
		"tenant_id": tenantID,
	}

	_, err := c.request(subject, request)
	return err
}

// MigrateSchema performs schema migration
func (c *Client) MigrateSchema(serviceName string, dsl interface{}) error {
	subject := "dal.schema.migrate"

	request := map[string]interface{}{
		"service": serviceName,
		"dsl":     dsl,
	}

	_, err := c.request(subject, request)
	return err
}

// request performs a NATS request-reply
func (c *Client) request(subject string, data interface{}) (*QueryResult, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	msg, err := c.nc.Request(subject, payload, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var response Response
	if err := json.Unmarshal(msg.Data, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("DAL error: %s", response.Error)
	}

	// Parse data based on response type
	result := &QueryResult{}

	// Check if it's a query response with data array and total
	if dataMap, ok := response.Data.(map[string]interface{}); ok {
		if data, hasData := dataMap["data"]; hasData {
			result.Data = data
		} else {
			result.Data = dataMap
		}

		if total, hasTotal := dataMap["total"]; hasTotal {
			if t, ok := total.(float64); ok {
				result.Total = int64(t)
			}
		}
	} else {
		result.Data = response.Data
	}

	return result, nil
}

// QueryBuilder helps build queries
type QueryBuilder struct {
	query map[string]interface{}
}

func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		query: make(map[string]interface{}),
	}
}

func (qb *QueryBuilder) Select(fields ...string) *QueryBuilder {
	qb.query["select"] = fields
	return qb
}

func (qb *QueryBuilder) Where(field, operator string, value interface{}) *QueryBuilder {
	if qb.query["where"] == nil {
		qb.query["where"] = []interface{}{}
	}

	conditions := qb.query["where"].([]interface{})
	conditions = append(conditions, map[string]interface{}{
		"field":    field,
		"operator": operator,
		"value":    value,
	})
	qb.query["where"] = conditions

	return qb
}

func (qb *QueryBuilder) OrderBy(field string, desc bool) *QueryBuilder {
	if qb.query["order_by"] == nil {
		qb.query["order_by"] = []interface{}{}
	}

	orders := qb.query["order_by"].([]interface{})
	orders = append(orders, map[string]interface{}{
		"field": field,
		"desc":  desc,
	})
	qb.query["order_by"] = orders

	return qb
}

func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.query["limit"] = limit
	return qb
}

func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	qb.query["offset"] = offset
	return qb
}

func (qb *QueryBuilder) WithRelations(relations ...string) *QueryBuilder {
	rels := make([]interface{}, len(relations))
	for i, rel := range relations {
		rels[i] = map[string]interface{}{
			"name": rel,
		}
	}
	qb.query["relations"] = rels
	return qb
}

func (qb *QueryBuilder) Build() map[string]interface{} {
	return qb.query
}

// Response types
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type QueryResult struct {
	Data  interface{} `json:"data"`
	Total int64       `json:"total"`
}
