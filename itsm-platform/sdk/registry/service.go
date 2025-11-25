package registry

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"itsm-platform/sdk/dal"
	"itsm-platform/sdk/dsl"
	"itsm-platform/sdk/graph"
	natspkg "itsm-platform/sdk/nats"
	"itsm-platform/sdk/schema"
)

// Service represents a complete service with all components
type Service struct {
	Graph        *dsl.ServiceGraph
	DB           *pgxpool.Pool
	Schema       *schema.Manager
	Events       *natspkg.EventManager
	GraphSync    *graph.SyncManager
	DALRegistry  map[string]*dal.GenericDAL
	HooksRegistry map[string]dal.Hooks
}

// Config for service initialization
type Config struct {
	DSLPath     string
	DatabaseURL string
	NATSUrl     string
}

// NewService bootstraps a complete service from DSL
func NewService(ctx context.Context, cfg Config) (*Service, error) {
	// 1. Parse DSL
	parser := dsl.NewParser()
	serviceGraph, err := parser.LoadService(cfg.DSLPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load DSL: %w", err)
	}

	// 2. Connect to PostgreSQL
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 3. Connect to NATS
	nc, err := nats.Connect(cfg.NATSUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// 4. Initialize Event Manager
	eventMgr, err := natspkg.NewEventManager(nc, serviceGraph)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize event manager: %w", err)
	}

	// 5. Initialize components
	svc := &Service{
		Graph:         serviceGraph,
		DB:            db,
		Schema:        schema.NewManager(db, serviceGraph),
		Events:        eventMgr,
		GraphSync:     graph.NewSyncManager(db, serviceGraph),
		DALRegistry:   make(map[string]*dal.GenericDAL),
		HooksRegistry: make(map[string]dal.Hooks),
	}

	// 6. Initialize DAL for each node
	for _, node := range serviceGraph.Nodes {
		entityDAL, err := dal.NewGenericDAL(db, serviceGraph, node.Name, eventMgr)
		if err != nil {
			return nil, fmt.Errorf("failed to create DAL for %s: %w", node.Name, err)
		}
		svc.DALRegistry[node.Name] = entityDAL
	}

	// 7. Initialize Graph
	if err := svc.GraphSync.InitGraph(ctx); err != nil {
		// Log warning but don't fail - graph might not be available
		fmt.Printf("Warning: Graph initialization failed: %v\n", err)
	}

	// 8. Register graph sync event handler
	svc.registerGraphSyncHandlers(ctx)

	return svc, nil
}

// registerGraphSyncHandlers sets up event handlers for graph synchronization
func (s *Service) registerGraphSyncHandlers(ctx context.Context) {
	// Subscribe to our own events for graph sync
	for _, pub := range s.Graph.Events.Publish {
		subject := pub
		s.Events.RegisterHandler(subject, func(ctx context.Context, event natspkg.Event) error {
			return s.GraphSync.HandleEvent(ctx, event)
		})
	}
}

// GetDAL returns the DAL for a specific entity
func (s *Service) GetDAL(entityName string) (*dal.GenericDAL, error) {
	d, ok := s.DALRegistry[entityName]
	if !ok {
		return nil, fmt.Errorf("DAL not found for entity: %s", entityName)
	}
	return d, nil
}

// SetHooks sets custom hooks for an entity
func (s *Service) SetHooks(entityName string, hooks dal.Hooks) error {
	d, err := s.GetDAL(entityName)
	if err != nil {
		return err
	}
	d.SetHooks(hooks)
	s.HooksRegistry[entityName] = hooks
	return nil
}

// CreateTenantSchema creates schema for a new tenant
func (s *Service) CreateTenantSchema(ctx context.Context, tenantID string) error {
	return s.Schema.CreateTenantSchema(ctx, tenantID)
}

// StartEventSubscribers starts listening for events from other services
func (s *Service) StartEventSubscribers(ctx context.Context) error {
	return s.Events.StartSubscribers(ctx)
}

// Close cleanly shuts down all connections
func (s *Service) Close() {
	s.Events.Close()
	s.DB.Close()
}

// RegisterEventHandler registers a handler for events from other services
func (s *Service) RegisterEventHandler(subject string, handler func(ctx context.Context, event natspkg.Event) error) {
	s.Events.RegisterHandler(subject, handler)
}

// CRUD convenience methods

// Create creates a new entity
func (s *Service) Create(ctx context.Context, entityName, tenantID string, data dal.Entity) (dal.Entity, error) {
	d, err := s.GetDAL(entityName)
	if err != nil {
		return nil, err
	}
	return d.Create(ctx, tenantID, data)
}

// GetByID retrieves an entity
func (s *Service) GetByID(ctx context.Context, entityName, tenantID, id string) (dal.Entity, error) {
	d, err := s.GetDAL(entityName)
	if err != nil {
		return nil, err
	}
	return d.GetByID(ctx, tenantID, id)
}

// Update updates an entity
func (s *Service) Update(ctx context.Context, entityName, tenantID, id string, data dal.Entity) (dal.Entity, error) {
	d, err := s.GetDAL(entityName)
	if err != nil {
		return nil, err
	}
	return d.Update(ctx, tenantID, id, data)
}

// Delete removes an entity
func (s *Service) Delete(ctx context.Context, entityName, tenantID, id string) error {
	d, err := s.GetDAL(entityName)
	if err != nil {
		return err
	}
	return d.Delete(ctx, tenantID, id)
}

// List retrieves entities with filtering
func (s *Service) List(ctx context.Context, entityName, tenantID string, opts dal.ListOptions) ([]dal.Entity, int64, error) {
	d, err := s.GetDAL(entityName)
	if err != nil {
		return nil, 0, err
	}
	return d.List(ctx, tenantID, opts)
}

// GraphQuery executes a Cypher query
func (s *Service) GraphQuery(ctx context.Context, cypher string) ([]map[string]interface{}, error) {
	return s.GraphSync.Query(ctx, cypher)
}
