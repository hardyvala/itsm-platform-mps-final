// =============================================================================
// GENERATED CODE - DO NOT EDIT
// Generated from: ticket/service.json
// =============================================================================

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"

	"itsm-platform/sdk/dal"
	"itsm-platform/sdk/hooks"
	"itsm-platform/sdk/parser"
	"itsm-platform/sdk/query"
)

// =============================================================================
// SERVICE CONFIGURATION (from DSL metadata)
// =============================================================================

const (
	ServiceName  = "ticket"
	DatabaseName = "ticket_db"
	ServicePort  = 8001
)

// =============================================================================
// DAL INSTANCES (one per node)
// =============================================================================

var (
	ticketDAL *dal.DAL
	commentDAL *dal.DAL
	graph *parser.ServiceGraph
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load DSL at runtime for dynamic access
	p := parser.NewParser()
	var err error
	graph, err = p.Parse("./dsl/apps/ticket/service.json")
	if err != nil {
		log.Fatalf("Failed to parse DSL: %v", err)
	}

	// Connect to PostgreSQL
	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Connect to NATS
	nc, err := nats.Connect(os.Getenv("NATS_URL"))
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()
	eventBus := NewNATSPublisher(nc)

	// Initialize DALs
	ticketDAL = initTicketDAL(db, eventBus)
	commentDAL = initCommentDAL(db, eventBus)

	// Setup HTTP routes
	mux := http.NewServeMux()
	setupRoutes(mux)

	// Start server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", ServicePort),
		Handler: mux,
	}

	go func() {
		log.Printf("🚀 %s service starting on port %d", ServiceName, ServicePort)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Shutting down...")
	server.Shutdown(ctx)
}

// =============================================================================
// DAL INITIALIZATION (with hooks from DSL)
// =============================================================================

func initTicketDAL(db *pgxpool.Pool, eventBus dal.EventPublisher) *dal.DAL {
	node := graph.GetNode("Ticket")
	
	dalNode := &dal.Node{
		Name:  node.Name,
		Table: node.Table,
		Properties: make([]dal.Property, len(node.Properties)),
		Relations:  make([]dal.Relation, len(node.Relations)),
		DALConfig: dal.DALConfig{
			SoftDelete:     true,
			OptimisticLock: true,
		},
	}
	
	for i, p := range node.Properties {
		defaultStr := ""
		if p.Default != nil {
			defaultStr = fmt.Sprintf("%v", p.Default)
		}
		dalNode.Properties[i] = dal.Property{
			Name: p.Name, Type: p.Type, Required: p.Required, Default: defaultStr,
		}
	}
	
	for i, r := range node.Relations {
		dalNode.Relations[i] = dal.Relation{
			Name: r.Name, Type: r.Type, TargetService: r.TargetService,
			TargetNode: r.TargetNode, LocalField: r.LocalField, TargetField: r.TargetField,
		}
	}

	hookExecutor := hooks.NewExecutor(node)
	
	// Register actions from DSL
	hookExecutor.RegisterAction("notify_customer", action_notify_customer)
	hookExecutor.RegisterAction("auto_assign", action_auto_assign)
	hookExecutor.RegisterAction("calculate_sla", action_calculate_sla)
	hookExecutor.RegisterAction("cleanup_attachments", action_cleanup_attachments)

	// Register triggers from DSL
	hookExecutor.RegisterTrigger("notify_status_change", trigger_notify_status_change)
	hookExecutor.RegisterTrigger("notify_assignment", trigger_notify_assignment)

	// Register checks from DSL

	return dal.NewDAL(db, dalNode, ServiceName, hookExecutor, eventBus)
}

func initCommentDAL(db *pgxpool.Pool, eventBus dal.EventPublisher) *dal.DAL {
	node := graph.GetNode("Comment")
	
	dalNode := &dal.Node{
		Name:  node.Name,
		Table: node.Table,
		Properties: make([]dal.Property, len(node.Properties)),
		Relations:  make([]dal.Relation, len(node.Relations)),
		DALConfig: dal.DALConfig{
			SoftDelete:     true,
			OptimisticLock: false,
		},
	}
	
	for i, p := range node.Properties {
		defaultStr := ""
		if p.Default != nil {
			defaultStr = fmt.Sprintf("%v", p.Default)
		}
		dalNode.Properties[i] = dal.Property{
			Name: p.Name, Type: p.Type, Required: p.Required, Default: defaultStr,
		}
	}
	
	for i, r := range node.Relations {
		dalNode.Relations[i] = dal.Relation{
			Name: r.Name, Type: r.Type, TargetService: r.TargetService,
			TargetNode: r.TargetNode, LocalField: r.LocalField, TargetField: r.TargetField,
		}
	}

	hookExecutor := hooks.NewExecutor(node)
	
	// Register actions from DSL
	hookExecutor.RegisterAction("update_ticket_timestamp", action_update_ticket_timestamp)
	hookExecutor.RegisterAction("notify_watchers", action_notify_watchers)

	// Register triggers from DSL

	// Register checks from DSL

	return dal.NewDAL(db, dalNode, ServiceName, hookExecutor, eventBus)
}


// =============================================================================
// HTTP ROUTES
// =============================================================================

func setupRoutes(mux *http.ServeMux) {
	// Ticket routes
	mux.HandleFunc("/api/v1/tickets", handleTicketList)
	mux.HandleFunc("/api/v1/tickets/", handleTicketByID)
	mux.HandleFunc("/api/v1/tickets/query", handleTicketQuery)
	// Comment routes
	mux.HandleFunc("/api/v1/comments", handleCommentList)
	mux.HandleFunc("/api/v1/comments/", handleCommentByID)
	mux.HandleFunc("/api/v1/comments/query", handleCommentQuery)
	
	// Health check
	mux.HandleFunc("/health", handleHealth)
}

func handleTicketList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		httpError(w, http.StatusBadRequest, "X-Tenant-ID required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		q := query.Query{From: "tickets", Limit: 20}
		entities, total, err := ticketDAL.Execute(ctx, tenantID, q)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpJSON(w, http.StatusOK, map[string]interface{}{"data": entities, "total": total})

	case http.MethodPost:
		var data dal.Entity
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			httpError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}
		entity, err := ticketDAL.Create(ctx, tenantID, data)
		if err != nil {
			httpError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		httpJSON(w, http.StatusCreated, entity)

	default:
		httpError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func handleTicketQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	ctx := r.Context()
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		httpError(w, http.StatusBadRequest, "X-Tenant-ID required")
		return
	}

	var q query.Query
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		httpError(w, http.StatusBadRequest, "Invalid query")
		return
	}
	q.From = "tickets"

	entities, total, err := ticketDAL.Execute(ctx, tenantID, q)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpJSON(w, http.StatusOK, map[string]interface{}{"data": entities, "total": total})
}

func handleTicketByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		httpError(w, http.StatusBadRequest, "X-Tenant-ID required")
		return
	}
	id := extractID(r.URL.Path)

	switch r.Method {
	case http.MethodGet:
		entity, err := ticketDAL.GetByID(ctx, tenantID, id)
		if err != nil {
			httpError(w, http.StatusNotFound, "Not found")
			return
		}
		httpJSON(w, http.StatusOK, entity)

	case http.MethodPut, http.MethodPatch:
		var data dal.Entity
		json.NewDecoder(r.Body).Decode(&data)
		entity, err := ticketDAL.Update(ctx, tenantID, id, data)
		if err != nil {
			httpError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		httpJSON(w, http.StatusOK, entity)

	case http.MethodDelete:
		if err := ticketDAL.Delete(ctx, tenantID, id); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		httpError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func handleCommentList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		httpError(w, http.StatusBadRequest, "X-Tenant-ID required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		q := query.Query{From: "comments", Limit: 20}
		entities, total, err := commentDAL.Execute(ctx, tenantID, q)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpJSON(w, http.StatusOK, map[string]interface{}{"data": entities, "total": total})

	case http.MethodPost:
		var data dal.Entity
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			httpError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}
		entity, err := commentDAL.Create(ctx, tenantID, data)
		if err != nil {
			httpError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		httpJSON(w, http.StatusCreated, entity)

	default:
		httpError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func handleCommentQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	ctx := r.Context()
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		httpError(w, http.StatusBadRequest, "X-Tenant-ID required")
		return
	}

	var q query.Query
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		httpError(w, http.StatusBadRequest, "Invalid query")
		return
	}
	q.From = "comments"

	entities, total, err := commentDAL.Execute(ctx, tenantID, q)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpJSON(w, http.StatusOK, map[string]interface{}{"data": entities, "total": total})
}

func handleCommentByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		httpError(w, http.StatusBadRequest, "X-Tenant-ID required")
		return
	}
	id := extractID(r.URL.Path)

	switch r.Method {
	case http.MethodGet:
		entity, err := commentDAL.GetByID(ctx, tenantID, id)
		if err != nil {
			httpError(w, http.StatusNotFound, "Not found")
			return
		}
		httpJSON(w, http.StatusOK, entity)

	case http.MethodPut, http.MethodPatch:
		var data dal.Entity
		json.NewDecoder(r.Body).Decode(&data)
		entity, err := commentDAL.Update(ctx, tenantID, id, data)
		if err != nil {
			httpError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		httpJSON(w, http.StatusOK, entity)

	case http.MethodDelete:
		if err := commentDAL.Delete(ctx, tenantID, id); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		httpError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	httpJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": ServiceName})
}

// =============================================================================
// UTILITIES
// =============================================================================

type NATSPublisher struct{ nc *nats.Conn }

func NewNATSPublisher(nc *nats.Conn) *NATSPublisher { return &NATSPublisher{nc: nc} }

func (p *NATSPublisher) Publish(ctx context.Context, subject string, data interface{}) error {
	payload, _ := json.Marshal(data)
	return p.nc.Publish(subject, payload)
}

func httpJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func httpError(w http.ResponseWriter, status int, msg string) {
	httpJSON(w, status, map[string]string{"error": msg})
}

func extractID(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}
