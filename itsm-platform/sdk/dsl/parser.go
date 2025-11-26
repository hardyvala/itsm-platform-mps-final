package dsl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ServiceGraph represents the complete DSL for a service
type ServiceGraph struct {
	Version  string          `json:"version"`
	Metadata Metadata        `json:"metadata"`
	Platform *PlatformConfig `json:"platform,omitempty"`
	Nodes    []Node          `json:"nodes"`
	Edges    []Edge          `json:"edges,omitempty"`
	Events   Events          `json:"events"`
}

type Metadata struct {
	Service string `json:"service"`
}

type ResourceConfig struct {
	Requests ResourceSpec `json:"requests,omitempty"`
	Limits   ResourceSpec `json:"limits,omitempty"`
}

type ResourceSpec struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// Platform configuration
type PlatformConfig struct {
	Defaults DefaultConfig `json:"defaults"`
}

type DefaultConfig struct {
	IDStrategy string           `json:"id_strategy"`
	IDFallback string           `json:"id_fallback"`
	Timestamps TimestampConfig  `json:"timestamps"`
	SoftDelete SoftDeleteConfig `json:"soft_delete"`
	Versioning VersionConfig    `json:"versioning"`
	Audit      AuditConfig      `json:"audit"`
}

type TimestampConfig struct {
	CreatedAt bool `json:"created_at"`
	UpdatedAt bool `json:"updated_at"`
	DeletedAt bool `json:"deleted_at"`
}

type SoftDeleteConfig struct {
	Enabled  bool   `json:"enabled"`
	Strategy string `json:"strategy"`
}

type VersionConfig struct {
	Enabled           bool   `json:"enabled"`
	Field             string `json:"field"`
	OptimisticLocking bool   `json:"optimistic_locking"`
}

type AuditConfig struct {
	Enabled      bool `json:"enabled"`
	TrackChanges bool `json:"track_changes"`
}

// Node represents an entity (graph node = DB table)
type Node struct {
	Name           string             `json:"name"`
	Properties     []Property         `json:"properties"`
	References     []Reference        `json:"references,omitempty"`
	ManyToMany     []ManyToManyConfig `json:"many_to_many,omitempty"`
	ComputedFields []ComputedField    `json:"computed_fields,omitempty"`
	Indexes        []Index            `json:"indexes,omitempty"`
	Validations    []ValidationRule   `json:"validations,omitempty"`
	DAL            DALConfig          `json:"dal"`
	Relations      []Relation         `json:"relations,omitempty"`
	Hooks          HookConfig         `json:"hooks,omitempty"`
	CRUD           CRUDConfig         `json:"crud,omitempty"`
	Views          []ViewConfig       `json:"views,omitempty"`
	Graph          GraphConfig        `json:"graph,omitempty"`
}

type Property struct {
	Name            string      `json:"name"`
	Type            string      `json:"type"`
	Primary         bool        `json:"primary,omitempty"`
	Generator       string      `json:"generator,omitempty"`
	Required        bool        `json:"required,omitempty"`
	Nullable        bool        `json:"nullable,omitempty"`
	Indexed         interface{} `json:"indexed,omitempty"` // can be bool or string
	Unique          bool        `json:"unique,omitempty"`
	UniquePerTenant bool        `json:"unique_per_tenant,omitempty"`
	Immutable       bool        `json:"immutable,omitempty"`
	Searchable      bool        `json:"searchable,omitempty"`
	TrackHistory    bool        `json:"track_history,omitempty"`
	MaxLength       int         `json:"max_length,omitempty"`
	Default         interface{} `json:"default,omitempty"`
	Values          []string    `json:"values,omitempty"` // For enum type
	Precision       int         `json:"precision,omitempty"`
	Scale           int         `json:"scale,omitempty"`
	Validation      string      `json:"validation,omitempty"`
	Description     string      `json:"description,omitempty"`
	AutoGenerate    *AutoGen    `json:"auto_generate,omitempty"`
}

type AutoGen struct {
	Strategy string `json:"strategy"`
	Format   string `json:"format"`
}

type Index struct {
	Name       string   `json:"name"`
	Fields     []string `json:"fields,omitempty"`
	Type       string   `json:"type,omitempty"`
	Expression string   `json:"expression,omitempty"`
	Unique     bool     `json:"unique,omitempty"`
	Where      string   `json:"where,omitempty"`
}

// New types for enhanced DSL
type Reference struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable,omitempty"`
	Indexed  string `json:"indexed,omitempty"`
	Service  string `json:"service,omitempty"`
	Entity   string `json:"entity,omitempty"`
	Field    string `json:"field,omitempty"`
	LocalRef string `json:"local_ref,omitempty"`
}

type ManyToManyConfig struct {
	Name          string     `json:"name"`
	TargetEntity  string     `json:"target_entity"`
	JunctionTable string     `json:"junction_table"`
	LocalKey      string     `json:"local_key"`
	ForeignKey    string     `json:"foreign_key"`
	ExtraFields   []Property `json:"extra_fields,omitempty"`
}

type ComputedField struct {
	Name       string `json:"name"`
	Formula    string `json:"formula"`
	ReturnType string `json:"return_type"`
}

type CRUDConfig struct {
	Create CRUDOperation `json:"create"`
	Update CRUDOperation `json:"update"`
	Delete CRUDOperation `json:"delete"`
	Read   CRUDOperation `json:"read"`
}

type CRUDOperation struct {
	GraphNode    bool     `json:"graph_node,omitempty"`
	GraphEdges   bool     `json:"graph_edges,omitempty"`
	EmitEvent    bool     `json:"emit_event,omitempty"`
	SyncGraph    bool     `json:"sync_graph,omitempty"`
	SoftDelete   bool     `json:"soft_delete,omitempty"`
	DefaultLimit int      `json:"default_limit,omitempty"`
	MaxLimit     int      `json:"max_limit,omitempty"`
	Include      []string `json:"include,omitempty"`
}

type ViewConfig struct {
	Name   string   `json:"name"`
	Filter string   `json:"filter"`
	Sort   []string `json:"sort"`
}

type DALConfig struct {
	SoftDelete     bool `json:"soft_delete"`
	OptimisticLock bool `json:"optimistic_lock"`
}

// Edge represents a relationship between nodes
type Edge struct {
	Name       string      `json:"name"`
	From       string      `json:"from"`
	To         string      `json:"to"`
	Type       string      `json:"type"` // one_to_many, many_to_one, many_to_many
	ForeignKey *ForeignKey `json:"foreign_key,omitempty"`
	External   *External   `json:"external,omitempty"`
}

type ForeignKey struct {
	Field    string `json:"field"`
	OnNode   string `json:"on_node"`
	OnDelete string `json:"on_delete"`
}

type External struct {
	Service string `json:"service"`
}

type Events struct {
	Stream    string           `json:"stream"`
	Publish   []PublishEvent   `json:"publish"`
	Subscribe []SubscribeEvent `json:"subscribe"`
}

type PublishEvent struct {
	Event   string `json:"event"`
	Subject string `json:"subject"`
}

type SubscribeEvent struct {
	Subject string `json:"subject"`
	Handler string `json:"handler"`
}

// Relation represents a relationship between entities
type Relation struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	TargetService string `json:"target_service"`
	TargetNode    string `json:"target_node"`
	LocalField    string `json:"local_field"`
	TargetField   string `json:"target_field"`
}

// HookConfig represents business logic hooks
type HookConfig struct {
	PreCreate  HookDefinition `json:"pre_create,omitempty"`
	PostCreate HookDefinition `json:"post_create,omitempty"`
	PreUpdate  HookDefinition `json:"pre_update,omitempty"`
	PostUpdate HookDefinition `json:"post_update,omitempty"`
	PreDelete  HookDefinition `json:"pre_delete,omitempty"`
	PostDelete HookDefinition `json:"post_delete,omitempty"`
}

type HookDefinition struct {
	Enabled     bool             `json:"enabled"`
	Validations []ValidationRule `json:"validations,omitempty"`
	Actions     []string         `json:"actions,omitempty"`
	Rules       []BusinessRule   `json:"rules,omitempty"`
	Triggers    []Trigger        `json:"triggers,omitempty"`
	Checks      []string         `json:"checks,omitempty"`
	PreventIf   []PreventRule    `json:"prevent_if,omitempty"`
}

type PreventRule struct {
	Condition string `json:"condition"`
	Message   string `json:"message"`
}

type ValidationRule struct {
	Field   string      `json:"field"`
	Rule    string      `json:"rule"`
	Value   interface{} `json:"value,omitempty"`
	Message string      `json:"message"`
}

type BusinessRule struct {
	Condition string `json:"condition"`
	Action    string `json:"action"`
	Message   string `json:"message"`
}

type Trigger struct {
	OnFieldChange string `json:"on_field_change"`
	Action        string `json:"action"`
}

// GraphConfig represents graph/visualization configuration
type GraphConfig struct {
	Label          string      `json:"label"`
	Sync           bool        `json:"sync,omitempty"`
	SyncProperties []string    `json:"sync_properties"`
	Edges          []GraphEdge `json:"edges"`
}

type GraphEdge struct {
	Type       string   `json:"type"`
	To         string   `json:"to"`
	Via        string   `json:"via,omitempty"`
	Direction  string   `json:"direction,omitempty"`
	Properties []string `json:"properties,omitempty"`
}

// Parser loads DSL from file
type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) LoadService(path string) (*ServiceGraph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read DSL file: %w", err)
	}

	var graph ServiceGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, fmt.Errorf("failed to parse DSL: %w", err)
	}

	return &graph, nil
}

func (p *Parser) LoadFromDirectory(dir string) (*ServiceGraph, error) {
	servicePath := filepath.Join(dir, "service.json")
	return p.LoadService(servicePath)
}

// Helper methods on ServiceGraph

func (g *ServiceGraph) GetNode(name string) *Node {
	for i := range g.Nodes {
		if g.Nodes[i].Name == name {
			return &g.Nodes[i]
		}
	}
	return nil
}

func (g *ServiceGraph) GetEdgesFrom(nodeName string) []Edge {
	var edges []Edge
	for _, e := range g.Edges {
		if e.From == nodeName {
			edges = append(edges, e)
		}
	}
	return edges
}

func (g *ServiceGraph) GetEdgesTo(nodeName string) []Edge {
	var edges []Edge
	for _, e := range g.Edges {
		if e.To == nodeName {
			edges = append(edges, e)
		}
	}
	return edges
}

// GetPrimaryKey returns the primary key property for a node
func (n *Node) GetPrimaryKey() *Property {
	for i := range n.Properties {
		if n.Properties[i].Primary {
			return &n.Properties[i]
		}
	}
	return nil
}

// GetRequiredProperties returns all required properties
func (n *Node) GetRequiredProperties() []Property {
	var props []Property
	for _, p := range n.Properties {
		if p.Required {
			props = append(props, p)
		}
	}
	return props
}
