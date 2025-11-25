package dsl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ServiceGraph represents the complete DSL for a service
type ServiceGraph struct {
	Version  string   `json:"version"`
	Kind     string   `json:"kind"`
	Metadata Metadata `json:"metadata"`
	Nodes    []Node   `json:"nodes"`
	Edges    []Edge   `json:"edges"`
	Events   Events   `json:"events"`
}

type Metadata struct {
	Service  string `json:"service"`
	Database string `json:"database"`
	Port     int    `json:"port"`
	Package  string `json:"package,omitempty"`
	Version  string `json:"version,omitempty"`
}

// Node represents an entity (graph node = DB table)
type Node struct {
	Name       string      `json:"name"`
	Table      string      `json:"table"`
	Properties []Property  `json:"properties"`
	Indexes    []Index     `json:"indexes"`
	DAL        DALConfig   `json:"dal"`
	Relations  []Relation  `json:"relations,omitempty"`
	Hooks      HookConfig  `json:"hooks,omitempty"`
	Graph      GraphConfig `json:"graph,omitempty"`
}

type Property struct {
	Name            string      `json:"name"`
	Type            string      `json:"type"`
	Primary         bool        `json:"primary,omitempty"`
	Required        bool        `json:"required,omitempty"`
	Indexed         bool        `json:"indexed,omitempty"`
	UniquePerTenant bool        `json:"unique_per_tenant,omitempty"`
	MaxLength       int         `json:"max_length,omitempty"`
	Default         interface{} `json:"default,omitempty"`
	Values          []string    `json:"values,omitempty"` // For enum type
	Precision       int         `json:"precision,omitempty"`
	Scale           int         `json:"scale,omitempty"`
}

type Index struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
	Unique bool     `json:"unique,omitempty"`
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
	SyncProperties []string    `json:"sync_properties"`
	Edges          []GraphEdge `json:"edges"`
}

type GraphEdge struct {
	Type string `json:"type"`
	To   string `json:"to"`
	Via  string `json:"via"`
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
