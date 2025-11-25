package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"itsm-platform/sdk/dsl"
	"itsm-platform/sdk/nats"
)

// SyncManager keeps Apache AGE graph in sync with PostgreSQL
type SyncManager struct {
	db       *pgxpool.Pool
	graph    *dsl.ServiceGraph
	graphName string
}

// NewSyncManager creates a graph sync manager
func NewSyncManager(db *pgxpool.Pool, graph *dsl.ServiceGraph) *SyncManager {
	return &SyncManager{
		db:        db,
		graph:     graph,
		graphName: "itsm_graph", // Shared graph across services
	}
}

// InitGraph creates the graph and labels from DSL
func (m *SyncManager) InitGraph(ctx context.Context) error {
	// Create graph if not exists
	_, err := m.db.Exec(ctx, fmt.Sprintf(`
		SELECT * FROM ag_catalog.create_graph('%s')
	`, m.graphName))
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to create graph: %w", err)
	}

	// Create vertex labels for each node
	for _, node := range m.graph.Nodes {
		_, err := m.db.Exec(ctx, fmt.Sprintf(`
			SELECT * FROM ag_catalog.create_vlabel('%s', '%s')
		`, m.graphName, node.Name))
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to create label %s: %w", node.Name, err)
		}
	}

	// Create edge labels for each edge
	for _, edge := range m.graph.Edges {
		_, err := m.db.Exec(ctx, fmt.Sprintf(`
			SELECT * FROM ag_catalog.create_elabel('%s', '%s')
		`, m.graphName, edge.Name))
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to create edge label %s: %w", edge.Name, err)
		}
	}

	return nil
}

// HandleEvent processes events and syncs to graph
func (m *SyncManager) HandleEvent(ctx context.Context, event nats.Event) error {
	switch event.Action {
	case "created":
		return m.createNode(ctx, event)
	case "updated":
		return m.updateNode(ctx, event)
	case "deleted":
		return m.deleteNode(ctx, event)
	}
	return nil
}

// createNode creates a vertex in the graph
func (m *SyncManager) createNode(ctx context.Context, event nats.Event) error {
	node := m.graph.GetNode(capitalize(event.Entity))
	if node == nil {
		return nil // Node not in this service's graph
	}

	// Build properties JSON
	props := m.buildProperties(node, event.Data)

	query := fmt.Sprintf(`
		SELECT * FROM cypher('%s', $$
			CREATE (n:%s %s)
			RETURN n
		$$) AS (n agtype)
	`, m.graphName, node.Name, props)

	_, err := m.db.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create graph node: %w", err)
	}

	// Create edges for relationships
	return m.createEdges(ctx, event)
}

// updateNode updates a vertex in the graph
func (m *SyncManager) updateNode(ctx context.Context, event nats.Event) error {
	node := m.graph.GetNode(capitalize(event.Entity))
	if node == nil {
		return nil
	}

	id, ok := event.Data["id"].(string)
	if !ok {
		return fmt.Errorf("missing id in event data")
	}

	// Build SET clause
	setClauses := m.buildSetClauses(node, event.Data)

	query := fmt.Sprintf(`
		SELECT * FROM cypher('%s', $$
			MATCH (n:%s {id: '%s'})
			SET %s
			RETURN n
		$$) AS (n agtype)
	`, m.graphName, node.Name, id, setClauses)

	_, err := m.db.Exec(ctx, query)
	return err
}

// deleteNode removes a vertex and its edges from the graph
func (m *SyncManager) deleteNode(ctx context.Context, event nats.Event) error {
	node := m.graph.GetNode(capitalize(event.Entity))
	if node == nil {
		return nil
	}

	id, ok := event.Data["id"].(string)
	if !ok {
		return fmt.Errorf("missing id in event data")
	}

	query := fmt.Sprintf(`
		SELECT * FROM cypher('%s', $$
			MATCH (n:%s {id: '%s'})
			DETACH DELETE n
		$$) AS (result agtype)
	`, m.graphName, node.Name, id)

	_, err := m.db.Exec(ctx, query)
	return err
}

// createEdges creates edges based on DSL relationships
func (m *SyncManager) createEdges(ctx context.Context, event nats.Event) error {
	node := m.graph.GetNode(capitalize(event.Entity))
	if node == nil {
		return nil
	}

	// Find edges where this node is the "from"
	edges := m.graph.GetEdgesFrom(node.Name)

	for _, edge := range edges {
		if edge.ForeignKey == nil {
			continue
		}

		// Get the foreign key value from event data
		fkValue, ok := event.Data[edge.ForeignKey.Field]
		if !ok || fkValue == nil {
			continue
		}

		fromID := event.Data["id"].(string)
		toID := fkValue.(string)

		// Skip external edges (handled by graph service)
		if edge.External != nil {
			continue
		}

		query := fmt.Sprintf(`
			SELECT * FROM cypher('%s', $$
				MATCH (a:%s {id: '%s'}), (b:%s {id: '%s'})
				CREATE (a)-[r:%s]->(b)
				RETURN r
			$$) AS (r agtype)
		`, m.graphName, edge.From, fromID, edge.To, toID, edge.Name)

		_, err := m.db.Exec(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to create edge %s: %w", edge.Name, err)
		}
	}

	return nil
}

// buildProperties builds Cypher properties object
func (m *SyncManager) buildProperties(node *dsl.Node, data map[string]interface{}) string {
	var props []string

	// Always include id and tenant_id
	if id, ok := data["id"]; ok {
		props = append(props, fmt.Sprintf("id: '%v'", id))
	}
	if tenantID, ok := data["tenant_id"]; ok {
		props = append(props, fmt.Sprintf("tenant_id: '%v'", tenantID))
	}

	// Add other properties from node definition
	for _, prop := range node.Properties {
		if prop.Name == "id" || prop.Name == "tenant_id" {
			continue
		}
		if val, ok := data[prop.Name]; ok && val != nil {
			switch prop.Type {
			case "text", "enum", "uuid":
				props = append(props, fmt.Sprintf("%s: '%v'", prop.Name, val))
			case "boolean":
				props = append(props, fmt.Sprintf("%s: %v", prop.Name, val))
			case "integer":
				props = append(props, fmt.Sprintf("%s: %v", prop.Name, val))
			default:
				props = append(props, fmt.Sprintf("%s: '%v'", prop.Name, val))
			}
		}
	}

	return "{" + strings.Join(props, ", ") + "}"
}

// buildSetClauses builds Cypher SET clauses for update
func (m *SyncManager) buildSetClauses(node *dsl.Node, data map[string]interface{}) string {
	var clauses []string

	for _, prop := range node.Properties {
		if prop.Name == "id" || prop.Name == "tenant_id" || prop.Primary {
			continue
		}
		if val, ok := data[prop.Name]; ok && val != nil {
			switch prop.Type {
			case "text", "enum", "uuid":
				clauses = append(clauses, fmt.Sprintf("n.%s = '%v'", prop.Name, val))
			default:
				clauses = append(clauses, fmt.Sprintf("n.%s = %v", prop.Name, val))
			}
		}
	}

	return strings.Join(clauses, ", ")
}

// Query executes a Cypher query on the graph
func (m *SyncManager) Query(ctx context.Context, cypher string) ([]map[string]interface{}, error) {
	query := fmt.Sprintf(`
		SELECT * FROM cypher('%s', $$
			%s
		$$) AS (result agtype)
	`, m.graphName, cypher)

	rows, err := m.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var result interface{}
		if err := rows.Scan(&result); err != nil {
			return nil, err
		}
		// Parse AGE result - simplified
		results = append(results, map[string]interface{}{"result": result})
	}

	return results, nil
}

// GetRelatedNodes finds nodes related to a given node
func (m *SyncManager) GetRelatedNodes(ctx context.Context, nodeType, nodeID string, edgeType string, direction string) ([]map[string]interface{}, error) {
	var cypher string

	if direction == "outgoing" {
		cypher = fmt.Sprintf(`
			MATCH (n:%s {id: '%s'})-[r:%s]->(m)
			RETURN m
		`, nodeType, nodeID, edgeType)
	} else {
		cypher = fmt.Sprintf(`
			MATCH (n:%s {id: '%s'})<-[r:%s]-(m)
			RETURN m
		`, nodeType, nodeID, edgeType)
	}

	return m.Query(ctx, cypher)
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
