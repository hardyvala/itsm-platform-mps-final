package main

import (
	"encoding/json"
	"fmt"
	"sync"
)

// ServiceRegistry manages all registered service DSLs
type ServiceRegistry struct {
	mu       sync.RWMutex
	services map[string]*ServiceDefinition
}

type ServiceDefinition struct {
	Name  string
	DSL   DSLDefinition
	Nodes map[string]*NodeDefinition
}

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string]*ServiceDefinition),
	}
}

// RegisterService registers a new service DSL
func (sr *ServiceRegistry) RegisterService(name string, dsl DSLDefinition) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	// Create service definition
	serviceDef := &ServiceDefinition{
		Name:  name,
		DSL:   dsl,
		Nodes: make(map[string]*NodeDefinition),
	}

	// Build node index for quick lookup
	for i := range dsl.Nodes {
		node := &dsl.Nodes[i]
		serviceDef.Nodes[node.Name] = node
	}

	sr.services[name] = serviceDef
	return nil
}

// GetService returns a service definition
func (sr *ServiceRegistry) GetService(name string) *ServiceDefinition {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.services[name]
}

// GetServiceDSL returns the DSL for a service
func (sr *ServiceRegistry) GetServiceDSL(name string) DSLDefinition {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	if service, ok := sr.services[name]; ok {
		return service.DSL
	}
	return DSLDefinition{}
}

// ListServices returns all registered service names
func (sr *ServiceRegistry) ListServices() []string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	names := make([]string, 0, len(sr.services))
	for name := range sr.services {
		names = append(names, name)
	}
	return names
}

// GetNode returns a specific node from a service
func (sd *ServiceDefinition) GetNode(nodeName string) *NodeDefinition {
	if sd == nil {
		return nil
	}
	return sd.Nodes[nodeName]
}

// GetEdgesFrom returns all edges originating from a node
func (sd *ServiceDefinition) GetEdgesFrom(nodeName string) []EdgeDefinition {
	var edges []EdgeDefinition
	for _, edge := range sd.DSL.Edges {
		if edge.From == nodeName {
			edges = append(edges, edge)
		}
	}
	return edges
}

// GetEdgesTo returns all edges pointing to a node
func (sd *ServiceDefinition) GetEdgesTo(nodeName string) []EdgeDefinition {
	var edges []EdgeDefinition
	for _, edge := range sd.DSL.Edges {
		if edge.To == nodeName {
			edges = append(edges, edge)
		}
	}
	return edges
}

// ValidateRelation checks if a relation is valid
func (sd *ServiceDefinition) ValidateRelation(fromNode, relationName string) (*EdgeDefinition, error) {
	for _, edge := range sd.DSL.Edges {
		if edge.From == fromNode && edge.Name == relationName {
			return &edge, nil
		}
	}
	return nil, fmt.Errorf("relation %s not found on node %s", relationName, fromNode)
}

// Serialize serializes the registry to JSON (for persistence)
func (sr *ServiceRegistry) Serialize() ([]byte, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return json.Marshal(sr.services)
}

// Deserialize loads the registry from JSON
func (sr *ServiceRegistry) Deserialize(data []byte) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	services := make(map[string]*ServiceDefinition)
	if err := json.Unmarshal(data, &services); err != nil {
		return err
	}

	sr.services = services
	return nil
}
