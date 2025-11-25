package main

import "encoding/json"

// Request/Response types for NATS communication

type RegisterRequest struct {
	Service string        `json:"service"`
	DSL     DSLDefinition `json:"dsl"`
}

type QueryRequest struct {
	TenantID string `json:"tenant_id"`
	Query    Query  `json:"query"`
}

type CreateRequest struct {
	TenantID string                 `json:"tenant_id"`
	Data     map[string]interface{} `json:"data"`
}

type UpdateRequest struct {
	TenantID string                 `json:"tenant_id"`
	ID       string                 `json:"id"`
	Data     map[string]interface{} `json:"data"`
}

type DeleteRequest struct {
	TenantID string `json:"tenant_id"`
	ID       string `json:"id"`
}

type GetRequest struct {
	TenantID string `json:"tenant_id"`
	ID       string `json:"id"`
}

type TenantRequest struct {
	TenantID string `json:"tenant_id"`
}

type MigrateRequest struct {
	Service string        `json:"service"`
	DSL     DSLDefinition `json:"dsl"`
}

// Query structure from UI/services
type Query struct {
	Select    []string        `json:"select,omitempty"`
	Where     []Condition     `json:"where,omitempty"`
	OrderBy   []OrderBy       `json:"order_by,omitempty"`
	Limit     int             `json:"limit,omitempty"`
	Offset    int             `json:"offset,omitempty"`
	Relations []RelationQuery `json:"relations,omitempty"`
}

type Condition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

type OrderBy struct {
	Field string `json:"field"`
	Desc  bool   `json:"desc,omitempty"`
}

type RelationQuery struct {
	Name   string      `json:"name"`
	Select []string    `json:"select,omitempty"`
	Where  []Condition `json:"where,omitempty"`
}

// DSL Definition structures
type DSLDefinition struct {
	Version  string           `json:"version"`
	Kind     string           `json:"kind"`
	Metadata ServiceMetadata  `json:"metadata"`
	Nodes    []NodeDefinition `json:"nodes"`
	Edges    []EdgeDefinition `json:"edges"`
}

type ServiceMetadata struct {
	Service string `json:"service"`
	Version string `json:"version"`
}

type NodeDefinition struct {
	Name       string               `json:"name"`
	Table      string               `json:"table"`
	Properties []PropertyDefinition `json:"properties"`
	Indexes    []IndexDefinition    `json:"indexes"`
	DAL        DALConfig            `json:"dal"`
}

type PropertyDefinition struct {
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	Required        bool     `json:"required,omitempty"`
	Indexed         bool     `json:"indexed,omitempty"`
	UniquePerTenant bool     `json:"unique_per_tenant,omitempty"`
	MaxLength       int      `json:"max_length,omitempty"`
	Default         string   `json:"default,omitempty"`
	Values          []string `json:"values,omitempty"`
	Precision       int      `json:"precision,omitempty"`
	Scale           int      `json:"scale,omitempty"`
}

type IndexDefinition struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
	Unique bool     `json:"unique,omitempty"`
}

type EdgeDefinition struct {
	Name     string `json:"name"`
	From     string `json:"from"`
	To       string `json:"to"`
	Type     string `json:"type"`
	LocalKey string `json:"local_key"`
	External bool   `json:"external,omitempty"`
	Service  string `json:"service,omitempty"`
}

type DALConfig struct {
	SoftDelete     bool `json:"soft_delete"`
	OptimisticLock bool `json:"optimistic_lock"`
}
