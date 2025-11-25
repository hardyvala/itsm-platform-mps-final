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
	Service     string              `json:"service"`
	Namespace   string              `json:"namespace,omitempty"`
	Database    string              `json:"database"`
	Host        string              `json:"host,omitempty"`
	Port        int                 `json:"port"`
	Replicas    int                 `json:"replicas,omitempty"`
	Package     string              `json:"package,omitempty"`
	Version     string              `json:"version,omitempty"`
	Description string              `json:"description,omitempty"`
	Resources   *ResourceDefinition `json:"resources,omitempty"`
	PoolSize    int                 `json:"pool_size,omitempty"`
}

type ResourceDefinition struct {
	Requests ResourceSpec `json:"requests,omitempty"`
	Limits   ResourceSpec `json:"limits,omitempty"`
}

type ResourceSpec struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
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
	Name           string                    `json:"name"`
	Table          string                    `json:"table"`
	Schema         string                    `json:"schema,omitempty"`
	Description    string                    `json:"description,omitempty"`
	Properties     []PropertyDefinition      `json:"properties"`
	References     []ReferenceDefinition     `json:"references,omitempty"`
	ManyToMany     []ManyToManyDefinition    `json:"many_to_many,omitempty"`
	ComputedFields []ComputedFieldDefinition `json:"computed_fields,omitempty"`
	Indexes        []IndexDefinition         `json:"indexes,omitempty"`
	Validations    []ValidationDefinition    `json:"validations,omitempty"`
	DAL            DALConfig                 `json:"dal"`
	Relations      []RelationDefinition      `json:"relations,omitempty"`
	Hooks          HookConfigDefinition      `json:"hooks,omitempty"`
	CRUD           CRUDConfigDefinition      `json:"crud,omitempty"`
	Views          []ViewDefinition          `json:"views,omitempty"`
	Graph          GraphConfigDefinition     `json:"graph,omitempty"`
}

type PropertyDefinition struct {
	Name            string             `json:"name"`
	Type            string             `json:"type"`
	Primary         bool               `json:"primary,omitempty"`
	Generator       string             `json:"generator,omitempty"`
	Required        bool               `json:"required,omitempty"`
	Nullable        bool               `json:"nullable,omitempty"`
	Indexed         interface{}        `json:"indexed,omitempty"` // can be bool or string
	Unique          bool               `json:"unique,omitempty"`
	UniquePerTenant bool               `json:"unique_per_tenant,omitempty"`
	Immutable       bool               `json:"immutable,omitempty"`
	Searchable      bool               `json:"searchable,omitempty"`
	TrackHistory    bool               `json:"track_history,omitempty"`
	MaxLength       int                `json:"max_length,omitempty"`
	Default         interface{}        `json:"default,omitempty"`
	Values          []string           `json:"values,omitempty"`
	Precision       int                `json:"precision,omitempty"`
	Scale           int                `json:"scale,omitempty"`
	Validation      string             `json:"validation,omitempty"`
	Description     string             `json:"description,omitempty"`
	AutoGenerate    *AutoGenDefinition `json:"auto_generate,omitempty"`
}

type AutoGenDefinition struct {
	Strategy string `json:"strategy"`
	Format   string `json:"format"`
}

type IndexDefinition struct {
	Name       string   `json:"name"`
	Fields     []string `json:"fields,omitempty"`
	Type       string   `json:"type,omitempty"`
	Expression string   `json:"expression,omitempty"`
	Unique     bool     `json:"unique,omitempty"`
	Where      string   `json:"where,omitempty"`
}

type ReferenceDefinition struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable,omitempty"`
	Indexed  string `json:"indexed,omitempty"`
	Service  string `json:"service,omitempty"`
	Entity   string `json:"entity,omitempty"`
	Field    string `json:"field,omitempty"`
	LocalRef string `json:"local_ref,omitempty"`
}

type ManyToManyDefinition struct {
	Name          string               `json:"name"`
	TargetEntity  string               `json:"target_entity"`
	JunctionTable string               `json:"junction_table"`
	LocalKey      string               `json:"local_key"`
	ForeignKey    string               `json:"foreign_key"`
	ExtraFields   []PropertyDefinition `json:"extra_fields,omitempty"`
}

type ComputedFieldDefinition struct {
	Name       string `json:"name"`
	Formula    string `json:"formula"`
	ReturnType string `json:"return_type"`
}

type CRUDConfigDefinition struct {
	Create CRUDOperationDefinition `json:"create"`
	Update CRUDOperationDefinition `json:"update"`
	Delete CRUDOperationDefinition `json:"delete"`
	Read   CRUDOperationDefinition `json:"read"`
}

type CRUDOperationDefinition struct {
	GraphNode    bool     `json:"graph_node,omitempty"`
	GraphEdges   bool     `json:"graph_edges,omitempty"`
	EmitEvent    bool     `json:"emit_event,omitempty"`
	SyncGraph    bool     `json:"sync_graph,omitempty"`
	SoftDelete   bool     `json:"soft_delete,omitempty"`
	DefaultLimit int      `json:"default_limit,omitempty"`
	MaxLimit     int      `json:"max_limit,omitempty"`
	Include      []string `json:"include,omitempty"`
}

type ViewDefinition struct {
	Name   string   `json:"name"`
	Filter string   `json:"filter"`
	Sort   []string `json:"sort"`
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
	Checks      []string                 `json:"checks,omitempty"`
	PreventIf   []PreventRuleDefinition  `json:"prevent_if,omitempty"`
}

type PreventRuleDefinition struct {
	Condition string `json:"condition"`
	Message   string `json:"message"`
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
	Sync           bool                  `json:"sync,omitempty"`
	SyncProperties []string              `json:"sync_properties"`
	Edges          []GraphEdgeDefinition `json:"edges"`
}

type GraphEdgeDefinition struct {
	Type       string   `json:"type"`
	To         string   `json:"to"`
	Via        string   `json:"via,omitempty"`
	Direction  string   `json:"direction,omitempty"`
	Properties []string `json:"properties,omitempty"`
}
