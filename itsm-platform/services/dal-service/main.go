package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

type Config struct {
	DatabaseURL string
	NatsURL     string
	ServiceName string
}

type DALService struct {
	db       *pgxpool.Pool
	nc       *nats.Conn
	config   Config
	registry *ServiceRegistry
	schemas  *SchemaManager
}

func main() {
	config := Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://itsm_user:itsm_pass@localhost/itsm_db"),
		NatsURL:     getEnv("NATS_URL", "nats://localhost:4222"),
		ServiceName: "dal-service",
	}

	db, err := pgxpool.New(context.Background(), config.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	nc, err := nats.Connect(config.NatsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	service := &DALService{
		db:       db,
		nc:       nc,
		config:   config,
		registry: NewServiceRegistry(),
		schemas:  NewSchemaManager(db),
	}

	if err := service.Start(); err != nil {
		log.Fatalf("Failed to start DAL service: %v", err)
	}

	// Create default tenant for development
	defaultTenant := getEnv("DEFAULT_TENANT_ID", "default")
	if err := service.schemas.CreateTenantSchema(context.Background(), defaultTenant); err != nil {
		log.Printf("Failed to create default tenant schema: %v", err)
	} else {
		log.Printf("Created default tenant schema: tenant_%s", defaultTenant)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down DAL service...")
}

func (s *DALService) Start() error {
	// Subscribe to DAL operations
	handlers := map[string]nats.MsgHandler{
		"dal.register":       s.handleRegister,
		"dal.*.*.query":      s.handleQuery,
		"dal.*.*.create":     s.handleCreate,
		"dal.*.*.update":     s.handleUpdate,
		"dal.*.*.delete":     s.handleDelete,
		"dal.*.*.get":        s.handleGet,
		"dal.tenant.create":  s.handleTenantCreate,
		"dal.schema.migrate": s.handleSchemaMigrate,
	}

	for subject, handler := range handlers {
		if _, err := s.nc.Subscribe(subject, handler); err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", subject, err)
		}
		log.Printf("Subscribed to %s", subject)
	}

	log.Println("DAL Service started successfully")
	return nil
}

func (s *DALService) handleRegister(msg *nats.Msg) {
	var req map[string]interface{}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		s.replyError(msg, err)
		return
	}

	service, ok := req["service"].(string)
	if !ok {
		s.replyError(msg, fmt.Errorf("invalid service name"))
		return
	}

	dsl, ok := req["dsl"]
	if !ok {
		s.replyError(msg, fmt.Errorf("missing DSL"))
		return
	}

	// Parse and register service DSL
	if err := s.registry.RegisterService(service, dsl); err != nil {
		s.replyError(msg, err)
		return
	}

	// Convert to DSLDefinition for schema creation
	var dslDef DSLDefinition
	dslBytes, _ := json.Marshal(dsl)
	if err := json.Unmarshal(dslBytes, &dslDef); err != nil {
		log.Printf("Failed to convert DSL for schema creation: %v", err)
	} else {
		// Generate schemas for existing tenants
		tenants, _ := s.schemas.ListTenants(context.Background())
		for _, tenant := range tenants {
			if err := s.schemas.CreateServiceSchema(context.Background(), tenant, service, dslDef); err != nil {
				log.Printf("Failed to create schema for tenant %s: %v", tenant, err)
			}
		}
	}

	s.replySuccess(msg, map[string]interface{}{
		"status":  "registered",
		"service": service,
	})
}

func (s *DALService) handleQuery(msg *nats.Msg) {
	// Parse subject: dal.{service}.{entity}.query
	parts := parseDSubject(msg.Subject)
	service := parts["service"]
	entity := parts["entity"]

	var req QueryRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		s.replyError(msg, err)
		return
	}

	// Get service definition
	serviceDef := s.registry.GetService(service)
	if serviceDef == nil {
		s.replyError(msg, fmt.Errorf("service %s not registered", service))
		return
	}

	// Execute query
	executor := NewQueryExecutor(s.db, serviceDef)
	results, total, err := executor.Execute(context.Background(), req.TenantID, entity, req.Query)
	if err != nil {
		s.replyError(msg, err)
		return
	}

	s.replySuccess(msg, map[string]interface{}{
		"data":  results,
		"total": total,
	})
}

func (s *DALService) handleCreate(msg *nats.Msg) {
	parts := parseDSubject(msg.Subject)
	service := parts["service"]
	entity := parts["entity"]

	var req CreateRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		s.replyError(msg, err)
		return
	}

	serviceDef := s.registry.GetService(service)
	if serviceDef == nil {
		s.replyError(msg, fmt.Errorf("service %s not registered", service))
		return
	}

	executor := NewQueryExecutor(s.db, serviceDef)
	result, err := executor.Create(context.Background(), req.TenantID, entity, req.Data)
	if err != nil {
		s.replyError(msg, err)
		return
	}

	// Publish event
	s.publishEvent("created", service, req.TenantID, entity, result)

	s.replySuccess(msg, result)
}

func (s *DALService) handleUpdate(msg *nats.Msg) {
	parts := parseDSubject(msg.Subject)
	service := parts["service"]
	entity := parts["entity"]

	var req UpdateRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		s.replyError(msg, err)
		return
	}

	serviceDef := s.registry.GetService(service)
	if serviceDef == nil {
		s.replyError(msg, fmt.Errorf("service %s not registered", service))
		return
	}

	executor := NewQueryExecutor(s.db, serviceDef)
	result, err := executor.Update(context.Background(), req.TenantID, entity, req.ID, req.Data)
	if err != nil {
		s.replyError(msg, err)
		return
	}

	// Publish event
	s.publishEvent("updated", service, req.TenantID, entity, result)

	s.replySuccess(msg, result)
}

func (s *DALService) handleDelete(msg *nats.Msg) {
	parts := parseDSubject(msg.Subject)
	service := parts["service"]
	entity := parts["entity"]

	var req DeleteRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		s.replyError(msg, err)
		return
	}

	serviceDef := s.registry.GetService(service)
	if serviceDef == nil {
		s.replyError(msg, fmt.Errorf("service %s not registered", service))
		return
	}

	executor := NewQueryExecutor(s.db, serviceDef)
	err := executor.Delete(context.Background(), req.TenantID, entity, req.ID)
	if err != nil {
		s.replyError(msg, err)
		return
	}

	// Publish event
	s.publishEvent("deleted", service, req.TenantID, entity, map[string]interface{}{"id": req.ID})

	s.replySuccess(msg, map[string]interface{}{"deleted": true})
}

func (s *DALService) handleGet(msg *nats.Msg) {
	parts := parseDSubject(msg.Subject)
	service := parts["service"]
	entity := parts["entity"]

	var req GetRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		s.replyError(msg, err)
		return
	}

	serviceDef := s.registry.GetService(service)
	if serviceDef == nil {
		s.replyError(msg, fmt.Errorf("service %s not registered", service))
		return
	}

	executor := NewQueryExecutor(s.db, serviceDef)
	result, err := executor.GetByID(context.Background(), req.TenantID, entity, req.ID)
	if err != nil {
		s.replyError(msg, err)
		return
	}

	s.replySuccess(msg, result)
}

func (s *DALService) handleTenantCreate(msg *nats.Msg) {
	var req TenantRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		s.replyError(msg, err)
		return
	}

	// Create tenant schema
	if err := s.schemas.CreateTenantSchema(context.Background(), req.TenantID); err != nil {
		s.replyError(msg, err)
		return
	}

	// Create tables for all registered services
	for _, service := range s.registry.ListServices() {
		dsl := s.registry.GetServiceDSL(service)
		if err := s.schemas.CreateServiceSchema(context.Background(), req.TenantID, service, dsl); err != nil {
			log.Printf("Failed to create schema for service %s: %v", service, err)
		}
	}

	s.replySuccess(msg, map[string]interface{}{
		"tenant_id": req.TenantID,
		"status":    "created",
	})
}

func (s *DALService) handleSchemaMigrate(msg *nats.Msg) {
	var req MigrateRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		s.replyError(msg, err)
		return
	}

	// Run migrations
	migrator := NewMigrator(s.db, s.registry)
	if err := migrator.Migrate(context.Background(), req.Service, req.DSL); err != nil {
		s.replyError(msg, err)
		return
	}

	s.replySuccess(msg, map[string]interface{}{
		"status": "migrated",
	})
}

func (s *DALService) publishEvent(action, service, tenantID, entity string, data interface{}) {
	subject := fmt.Sprintf("%s.%s.%s.%s", service, tenantID, entity, action)
	payload, _ := json.Marshal(map[string]interface{}{
		"action":    action,
		"service":   service,
		"tenant_id": tenantID,
		"entity":    entity,
		"data":      data,
	})
	s.nc.Publish(subject, payload)
}

func (s *DALService) replySuccess(msg *nats.Msg, data interface{}) {
	response := map[string]interface{}{
		"success": true,
		"data":    data,
	}
	payload, _ := json.Marshal(response)
	msg.Respond(payload)
}

func (s *DALService) replyError(msg *nats.Msg, err error) {
	response := map[string]interface{}{
		"success": false,
		"error":   err.Error(),
	}
	payload, _ := json.Marshal(response)
	msg.Respond(payload)
}

func parseDSubject(subject string) map[string]string {
	// dal.{service}.{entity}.{action}
	parts := make(map[string]string)
	tokens := strings.Split(subject, ".")
	if len(tokens) >= 4 {
		parts["service"] = tokens[1]
		parts["entity"] = tokens[2]
		parts["action"] = tokens[3]
	}
	return parts
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
