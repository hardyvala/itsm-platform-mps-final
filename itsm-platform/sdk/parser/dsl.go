package parser

import (
	"encoding/json"
	"fmt"
	"os"
)

// ServiceGraph is the root DSL structure
type ServiceGraph struct {
	Version  string   `json:"version"`
	Kind     string   `json:"kind"`
	Metadata Metadata `json:"metadata"`
	Nodes    []Node   `json:"nodes"`
	Events   Events   `json:"events"`
}

type Metadata struct {
	Service  string `json:"service"`
	Database string `json:"database"`
	Port     int    `json:"port"`
	Package  string `json:"package"`
}

// Node represents an entity in the graph
type Node struct {
	Name       string     `json:"name"`
	Table      string     `json:"table"`
	Properties []Property `json:"properties"`
	Indexes    []Index    `json:"indexes"`
	DAL        DALConfig  `json:"dal"`
	Relations  []Relation `json:"relations"`
	Hooks      Hooks      `json:"hooks"`
	Graph      GraphConfig `json:"graph"`
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
	Values          []string    `json:"values,omitempty"` // For enum
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

// Relation defines relationship (no FK, just ID mapping)
type Relation struct {
	Name          string `json:"name"`
	Type          string `json:"type"` // belongs_to, has_many, has_one
	TargetService string `json:"target_service"`
	TargetNode    string `json:"target_node"`
	LocalField    string `json:"local_field"`
	TargetField   string `json:"target_field"`
}

// Hooks defines lifecycle hooks from DSL
type Hooks struct {
	PreCreate  HookConfig `json:"pre_create"`
	PostCreate HookConfig `json:"post_create"`
	PreUpdate  HookConfig `json:"pre_update"`
	PostUpdate HookConfig `json:"post_update"`
	PreDelete  HookConfig `json:"pre_delete"`
	PostDelete HookConfig `json:"post_delete"`
}

type HookConfig struct {
	Enabled     bool         `json:"enabled"`
	Validations []Validation `json:"validations,omitempty"`
	Rules       []Rule       `json:"rules,omitempty"`
	Actions     []string     `json:"actions,omitempty"`
	Triggers    []Trigger    `json:"triggers,omitempty"`
	Checks      []string     `json:"checks,omitempty"`
}

type Validation struct {
	Field   string      `json:"field"`
	Rule    string      `json:"rule"` // required, min_length, max_length, email_format, regex, etc.
	Value   interface{} `json:"value,omitempty"`
	Message string      `json:"message"`
}

type Rule struct {
	Condition string `json:"condition"` // Expression like "old.status == 'closed'"
	Action    string `json:"action"`    // reject, warn, transform
	Message   string `json:"message,omitempty"`
}

type Trigger struct {
	OnFieldChange string `json:"on_field_change"`
	Action        string `json:"action"`
}

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

type Events struct {
	Stream    string          `json:"stream"`
	Publish   []PublishEvent  `json:"publish"`
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

// Parser loads and parses DSL files
type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(path string) (*ServiceGraph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read DSL: %w", err)
	}

	var graph ServiceGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, fmt.Errorf("failed to parse DSL: %w", err)
	}

	return &graph, nil
}

// GetNode returns a node by name
func (g *ServiceGraph) GetNode(name string) *Node {
	for i := range g.Nodes {
		if g.Nodes[i].Name == name {
			return &g.Nodes[i]
		}
	}
	return nil
}

// GetPropertyNames returns all property names for a node
func (n *Node) GetPropertyNames() []string {
	names := make([]string, len(n.Properties))
	for i, p := range n.Properties {
		names[i] = p.Name
	}
	return names
}

// HasHook checks if a specific hook is enabled
func (n *Node) HasHook(hookType string) bool {
	switch hookType {
	case "pre_create":
		return n.Hooks.PreCreate.Enabled
	case "post_create":
		return n.Hooks.PostCreate.Enabled
	case "pre_update":
		return n.Hooks.PreUpdate.Enabled
	case "post_update":
		return n.Hooks.PostUpdate.Enabled
	case "pre_delete":
		return n.Hooks.PreDelete.Enabled
	case "post_delete":
		return n.Hooks.PostDelete.Enabled
	}
	return false
}

// GetValidations returns validations for pre_create or pre_update
func (n *Node) GetValidations(hookType string) []Validation {
	switch hookType {
	case "pre_create":
		return n.Hooks.PreCreate.Validations
	case "pre_update":
		return n.Hooks.PreUpdate.Validations
	}
	return nil
}

// GetActions returns actions for a hook
func (n *Node) GetActions(hookType string) []string {
	switch hookType {
	case "post_create":
		return n.Hooks.PostCreate.Actions
	case "post_update":
		return n.Hooks.PostUpdate.Actions
	case "post_delete":
		return n.Hooks.PostDelete.Actions
	}
	return nil
}

// GetTriggers returns triggers for a hook
func (n *Node) GetTriggers(hookType string) []Trigger {
	switch hookType {
	case "post_update":
		return n.Hooks.PostUpdate.Triggers
	}
	return nil
}

// GetDefaultValue returns the default value as a string
func (p *Property) GetDefaultValue() string {
	if p.Default == nil {
		return ""
	}
	return fmt.Sprintf("%v", p.Default)
}
