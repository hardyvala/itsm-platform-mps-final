package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// =============================================================================
// DSL STRUCTURES (same as sdk/parser)
// =============================================================================

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

type Node struct {
	Name       string     `json:"name"`
	Table      string     `json:"table"`
	Properties []Property `json:"properties"`
	Relations  []Relation `json:"relations"`
	DAL        DALConfig  `json:"dal"`
	Hooks      Hooks      `json:"hooks"`
}

type Property struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Required bool        `json:"required"`
	Default  interface{} `json:"default"`
}

type Relation struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	TargetService string `json:"target_service"`
	TargetNode    string `json:"target_node"`
	LocalField    string `json:"local_field"`
	TargetField   string `json:"target_field"`
}

type DALConfig struct {
	SoftDelete     bool `json:"soft_delete"`
	OptimisticLock bool `json:"optimistic_lock"`
}

type Hooks struct {
	PreCreate  HookConfig `json:"pre_create"`
	PostCreate HookConfig `json:"post_create"`
	PreUpdate  HookConfig `json:"pre_update"`
	PostUpdate HookConfig `json:"post_update"`
	PreDelete  HookConfig `json:"pre_delete"`
	PostDelete HookConfig `json:"post_delete"`
}

type HookConfig struct {
	Enabled  bool      `json:"enabled"`
	Actions  []string  `json:"actions"`
	Triggers []Trigger `json:"triggers"`
	Checks   []string  `json:"checks"`
}

type Trigger struct {
	OnFieldChange string `json:"on_field_change"`
	Action        string `json:"action"`
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

// =============================================================================
// CODE GENERATOR
// =============================================================================

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: codegen <dsl-path> <output-dir>")
		fmt.Println("Example: codegen dsl/apps/ticket/service.json generated/ticket-service")
		os.Exit(1)
	}

	dslPath := os.Args[1]
	outputDir := os.Args[2]

	// Parse DSL
	graph, err := parseDSL(dslPath)
	if err != nil {
		fmt.Printf("Error parsing DSL: %v\n", err)
		os.Exit(1)
	}

	// Generate code
	if err := generateService(graph, outputDir); err != nil {
		fmt.Printf("Error generating code: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Generated %s-service in %s\n", graph.Metadata.Service, outputDir)
}

func parseDSL(path string) (*ServiceGraph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var graph ServiceGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, err
	}

	return &graph, nil
}

func generateService(graph *ServiceGraph, outputDir string) error {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// Generate main.go
	mainCode, err := generateMainGo(graph)
	if err != nil {
		return err
	}

	mainPath := filepath.Join(outputDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(mainCode), 0644); err != nil {
		return err
	}

	// Generate actions.go (stubs for developer to implement)
	actionsCode, err := generateActionsGo(graph)
	if err != nil {
		return err
	}

	actionsPath := filepath.Join(outputDir, "actions.go")
	if err := os.WriteFile(actionsPath, []byte(actionsCode), 0644); err != nil {
		return err
	}

	// Generate handlers.go (event handlers)
	handlersCode, err := generateHandlersGo(graph)
	if err != nil {
		return err
	}

	handlersPath := filepath.Join(outputDir, "handlers.go")
	if err := os.WriteFile(handlersPath, []byte(handlersCode), 0644); err != nil {
		return err
	}

	return nil
}

// =============================================================================
// TEMPLATES
// =============================================================================

const mainTemplate = `// =============================================================================
// GENERATED CODE - DO NOT EDIT
// Generated from: {{.Metadata.Service}}/service.json
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
	ServiceName  = "{{.Metadata.Service}}"
	DatabaseName = "{{.Metadata.Database}}"
	ServicePort  = {{.Metadata.Port}}
)

// =============================================================================
// DAL INSTANCES (one per node)
// =============================================================================

var (
{{- range .Nodes}}
	{{.Name | lower}}DAL *dal.DAL
{{- end}}
	graph *parser.ServiceGraph
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load DSL at runtime for dynamic access
	p := parser.NewParser()
	var err error
	graph, err = p.Parse("./dsl/apps/{{.Metadata.Service}}/service.json")
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
{{- range .Nodes}}
	{{.Name | lower}}DAL = init{{.Name}}DAL(db, eventBus)
{{- end}}

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
{{range .Nodes}}
func init{{.Name}}DAL(db *pgxpool.Pool, eventBus dal.EventPublisher) *dal.DAL {
	node := graph.GetNode("{{.Name}}")
	
	dalNode := &dal.Node{
		Name:  node.Name,
		Table: node.Table,
		Properties: make([]dal.Property, len(node.Properties)),
		Relations:  make([]dal.Relation, len(node.Relations)),
		DALConfig: dal.DALConfig{
			SoftDelete:     {{.DAL.SoftDelete}},
			OptimisticLock: {{.DAL.OptimisticLock}},
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
{{- if .Hooks.PostCreate.Actions}}
{{- range .Hooks.PostCreate.Actions}}
	hookExecutor.RegisterAction("{{.}}", action_{{.}})
{{- end}}
{{- end}}
{{- if .Hooks.PostUpdate.Actions}}
{{- range .Hooks.PostUpdate.Actions}}
	hookExecutor.RegisterAction("{{.}}", action_{{.}})
{{- end}}
{{- end}}
{{- if .Hooks.PostDelete.Actions}}
{{- range .Hooks.PostDelete.Actions}}
	hookExecutor.RegisterAction("{{.}}", action_{{.}})
{{- end}}
{{- end}}

	// Register triggers from DSL
{{- if .Hooks.PostUpdate.Triggers}}
{{- range .Hooks.PostUpdate.Triggers}}
	hookExecutor.RegisterTrigger("{{.Action}}", trigger_{{.Action}})
{{- end}}
{{- end}}

	// Register checks from DSL
{{- if .Hooks.PreDelete.Checks}}
{{- range .Hooks.PreDelete.Checks}}
	hookExecutor.RegisterCheck("{{.}}", check_{{.}})
{{- end}}
{{- end}}

	return dal.NewDAL(db, dalNode, ServiceName, hookExecutor, eventBus)
}
{{end}}

// =============================================================================
// HTTP ROUTES
// =============================================================================

func setupRoutes(mux *http.ServeMux) {
{{- range .Nodes}}
	// {{.Name}} routes
	mux.HandleFunc("/api/v1/{{.Table}}", handle{{.Name}}List)
	mux.HandleFunc("/api/v1/{{.Table}}/", handle{{.Name}}ByID)
	mux.HandleFunc("/api/v1/{{.Table}}/query", handle{{.Name}}Query)
{{- end}}
	
	// Health check
	mux.HandleFunc("/health", handleHealth)
}
{{range .Nodes}}
func handle{{.Name}}List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		httpError(w, http.StatusBadRequest, "X-Tenant-ID required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		q := query.Query{From: "{{.Table}}", Limit: 20}
		entities, total, err := {{.Name | lower}}DAL.Execute(ctx, tenantID, q)
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
		entity, err := {{.Name | lower}}DAL.Create(ctx, tenantID, data)
		if err != nil {
			httpError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		httpJSON(w, http.StatusCreated, entity)

	default:
		httpError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func handle{{.Name}}Query(w http.ResponseWriter, r *http.Request) {
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
	q.From = "{{.Table}}"

	entities, total, err := {{.Name | lower}}DAL.Execute(ctx, tenantID, q)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpJSON(w, http.StatusOK, map[string]interface{}{"data": entities, "total": total})
}

func handle{{.Name}}ByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		httpError(w, http.StatusBadRequest, "X-Tenant-ID required")
		return
	}
	id := extractID(r.URL.Path)

	switch r.Method {
	case http.MethodGet:
		entity, err := {{.Name | lower}}DAL.GetByID(ctx, tenantID, id)
		if err != nil {
			httpError(w, http.StatusNotFound, "Not found")
			return
		}
		httpJSON(w, http.StatusOK, entity)

	case http.MethodPut, http.MethodPatch:
		var data dal.Entity
		json.NewDecoder(r.Body).Decode(&data)
		entity, err := {{.Name | lower}}DAL.Update(ctx, tenantID, id, data)
		if err != nil {
			httpError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		httpJSON(w, http.StatusOK, entity)

	case http.MethodDelete:
		if err := {{.Name | lower}}DAL.Delete(ctx, tenantID, id); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		httpError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}
{{end}}
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
`

const actionsTemplate = `// =============================================================================
// ACTION HANDLERS - IMPLEMENT YOUR BUSINESS LOGIC HERE
// Generated from: {{.Metadata.Service}}/service.json
// =============================================================================

package main

import (
	"context"
	"log"

	"itsm-platform/sdk/dal"
)

// =============================================================================
// POST-CREATE ACTIONS
// =============================================================================
{{range $node := .Nodes}}
{{- if $node.Hooks.PostCreate.Actions}}
// Actions for {{$node.Name}}
{{- range $node.Hooks.PostCreate.Actions}}

// action_{{.}} - Called after {{$.Metadata.Service}}.{{$node.Name}} is created
// TODO: Implement your business logic
func action_{{.}}(ctx context.Context, entity dal.Entity) error {
	log.Printf("ACTION [{{.}}]: entity_id=%v", entity["id"])
	// TODO: Implement {{.}}
	return nil
}
{{- end}}
{{- end}}
{{- end}}

// =============================================================================
// POST-UPDATE ACTIONS
// =============================================================================
{{range $node := .Nodes}}
{{- if $node.Hooks.PostUpdate.Actions}}
{{- range $node.Hooks.PostUpdate.Actions}}

func action_{{.}}(ctx context.Context, entity dal.Entity) error {
	log.Printf("ACTION [{{.}}]: entity_id=%v", entity["id"])
	// TODO: Implement {{.}}
	return nil
}
{{- end}}
{{- end}}
{{- end}}

// =============================================================================
// POST-DELETE ACTIONS
// =============================================================================
{{range $node := .Nodes}}
{{- if $node.Hooks.PostDelete.Actions}}
{{- range $node.Hooks.PostDelete.Actions}}

func action_{{.}}(ctx context.Context, entity dal.Entity) error {
	log.Printf("ACTION [{{.}}]: entity_id=%v", entity["id"])
	// TODO: Implement {{.}}
	return nil
}
{{- end}}
{{- end}}
{{- end}}

// =============================================================================
// TRIGGERS (on field change)
// =============================================================================
{{range $node := .Nodes}}
{{- if $node.Hooks.PostUpdate.Triggers}}
{{- range $node.Hooks.PostUpdate.Triggers}}

// trigger_{{.Action}} - Called when {{.OnFieldChange}} changes on {{$node.Name}}
func trigger_{{.Action}}(ctx context.Context, old, new dal.Entity, field string) error {
	log.Printf("TRIGGER [{{.Action}}]: %s changed from %v to %v", 
		field, old[field], new[field])
	// TODO: Implement {{.Action}}
	return nil
}
{{- end}}
{{- end}}
{{- end}}

// =============================================================================
// PRE-DELETE CHECKS
// =============================================================================
{{range $node := .Nodes}}
{{- if $node.Hooks.PreDelete.Checks}}
{{- range $node.Hooks.PreDelete.Checks}}

// check_{{.}} - Validates before {{$node.Name}} can be deleted
func check_{{.}}(ctx context.Context, entity dal.Entity) error {
	log.Printf("CHECK [{{.}}]: entity_id=%v", entity["id"])
	// TODO: Implement {{.}} - return error to prevent deletion
	return nil
}
{{- end}}
{{- end}}
{{- end}}
`

const handlersTemplate = `// =============================================================================
// EVENT HANDLERS - FOR CROSS-SERVICE EVENTS
// Generated from: {{.Metadata.Service}}/service.json
// =============================================================================

package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/nats-io/nats.go"
)

// =============================================================================
// SUBSCRIBE HANDLERS (from other services)
// =============================================================================
{{if .Events.Subscribe}}
{{range .Events.Subscribe}}
// {{.Handler}} - Handles events from: {{.Subject}}
func {{.Handler}}(ctx context.Context, event nats.Event) error {
	log.Printf("EVENT [{{.Handler}}]: received from %s, tenant=%s", 
		event.Subject, event.TenantID)
	
	// TODO: Implement handler for {{.Subject}}
	// event.Data contains the entity data
	
	return nil
}
{{end}}
{{else}}
// No subscriptions defined for this service
{{end}}

// =============================================================================
// REGISTER HANDLERS (call this in main if needed)
// =============================================================================

func registerEventHandlers(em *nats.EventManager) {
{{- if .Events.Subscribe}}
{{- range .Events.Subscribe}}
	em.RegisterHandler("{{.Subject}}", {{.Handler}})
{{- end}}
{{- else}}
	// No subscriptions to register
{{- end}}
}
`

func generateMainGo(graph *ServiceGraph) (string, error) {
	funcMap := template.FuncMap{
		"lower": strings.ToLower,
	}

	tmpl, err := template.New("main").Funcs(funcMap).Parse(mainTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, graph); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func generateActionsGo(graph *ServiceGraph) (string, error) {
	tmpl, err := template.New("actions").Parse(actionsTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, graph); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func generateHandlersGo(graph *ServiceGraph) (string, error) {
	tmpl, err := template.New("handlers").Parse(handlersTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, graph); err != nil {
		return "", err
	}

	return buf.String(), nil
}
