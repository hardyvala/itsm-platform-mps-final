package main

// Request/Response types for NATS communication

// RegisterRequest is now handled dynamically as map[string]interface{}

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
	Edges    []EdgeDefinition `json:"edges,omitempty"`
	Events   EventsDefinition `json:"events,omitempty"`
}

type ServiceMetadata struct {
	Service  string `json:"service"`
	Database string `json:"database"`
	Port     int    `json:"port"`
	Package  string `json:"package,omitempty"`
	Version  string `json:"version,omitempty"`
}

type EventsDefinition struct {
	Stream    string                     `json:"stream"`
	Publish   []PublishEventDefinition   `json:"publish"`
	Subscribe []SubscribeEventDefinition `json:"subscribe"`
}

type PublishEventDefinition struct {
	Event   string `json:"event"`
	Subject string `json:"subject"`
}

type SubscribeEventDefinition struct {
	Subject string `json:"subject"`
	Handler string `json:"handler"`
}

type NodeDefinition struct {
	Name       string                `json:"name"`
	Table      string                `json:"table"`
	Properties []PropertyDefinition  `json:"properties"`
	Indexes    []IndexDefinition     `json:"indexes"`
	DAL        DALConfig             `json:"dal"`
	Relations  []RelationDefinition  `json:"relations,omitempty"`
	Hooks      HookConfigDefinition  `json:"hooks,omitempty"`
	Graph      GraphConfigDefinition `json:"graph,omitempty"`
}

type PropertyDefinition struct {
	Name            string      `json:"name"`
	Type            string      `json:"type"`
	Primary         bool        `json:"primary,omitempty"`
	Required        bool        `json:"required,omitempty"`
	Indexed         bool        `json:"indexed,omitempty"`
	UniquePerTenant bool        `json:"unique_per_tenant,omitempty"`
	MaxLength       int         `json:"max_length,omitempty"`
	Default         interface{} `json:"default,omitempty"`
	Values          []string    `json:"values,omitempty"`
	Precision       int         `json:"precision,omitempty"`
	Scale           int         `json:"scale,omitempty"`
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

// Additional types to match SDK structure
type RelationDefinition struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	TargetService string `json:"target_service"`
	TargetNode    string `json:"target_node"`
	LocalField    string `json:"local_field"`
	TargetField   string `json:"target_field"`
}

type HookConfigDefinition struct {
	PreCreate  HookDefinition `json:"pre_create,omitempty"`
	PostCreate HookDefinition `json:"post_create,omitempty"`
	PreUpdate  HookDefinition `json:"pre_update,omitempty"`
	PostUpdate HookDefinition `json:"post_update,omitempty"`
	PreDelete  HookDefinition `json:"pre_delete,omitempty"`
	PostDelete HookDefinition `json:"post_delete,omitempty"`
}

type HookDefinition struct {
	Enabled     bool                     `json:"enabled"`
	Validations []ValidationDefinition   `json:"validations,omitempty"`
	Actions     []string                 `json:"actions,omitempty"`
	Rules       []BusinessRuleDefinition `json:"rules,omitempty"`
	Triggers    []TriggerDefinition      `json:"triggers,omitempty"`
}

type ValidationDefinition struct {
	Field   string      `json:"field"`
	Rule    string      `json:"rule"`
	Value   interface{} `json:"value,omitempty"`
	Message string      `json:"message"`
}

type BusinessRuleDefinition struct {
	Condition string `json:"condition"`
	Action    string `json:"action"`
	Message   string `json:"message"`
}

type TriggerDefinition struct {
	OnFieldChange string `json:"on_field_change"`
	Action        string `json:"action"`
}

type GraphConfigDefinition struct {
	Label          string                `json:"label"`
	SyncProperties []string              `json:"sync_properties"`
	Edges          []GraphEdgeDefinition `json:"edges"`
}

type GraphEdgeDefinition struct {
	Type string `json:"type"`
	To   string `json:"to"`
	Via  string `json:"via"`
}
