package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"itsm-platform/sdk/dsl"
)

// ServiceGenerator generates Go services from DSL definitions
type ServiceGenerator struct {
	parser *dsl.Parser
}

func NewServiceGenerator() *ServiceGenerator {
	return &ServiceGenerator{
		parser: dsl.NewParser(),
	}
}

// GenerateService creates a complete Go service from DSL
func (g *ServiceGenerator) GenerateService(dslPath, outputDir string) error {
	// Load DSL
	graph, err := g.parser.LoadService(dslPath)
	if err != nil {
		return fmt.Errorf("failed to load DSL: %w", err)
	}

	serviceName := graph.Metadata.Service
	serviceDir := filepath.Join(outputDir, serviceName+"-service")

	// Create service directory structure
	if err := g.createDirectories(serviceDir); err != nil {
		return err
	}

	// Generate files
	if err := g.generateMainFile(serviceDir, graph); err != nil {
		return err
	}

	if err := g.generateHandlers(serviceDir, graph); err != nil {
		return err
	}

	if err := g.generateTypes(serviceDir, graph); err != nil {
		return err
	}

	if err := g.generateDockerfile(serviceDir, serviceName); err != nil {
		return err
	}

	if err := g.copyDSLFile(dslPath, serviceDir); err != nil {
		return err
	}

	if err := g.generateGoMod(serviceDir, graph); err != nil {
		return err
	}

	if err := g.generateReadme(serviceDir, graph); err != nil {
		return err
	}

	fmt.Printf("Generated service: %s\n", serviceDir)
	return nil
}

func (g *ServiceGenerator) createDirectories(serviceDir string) error {
	dirs := []string{
		serviceDir,
		filepath.Join(serviceDir, "handlers"),
		filepath.Join(serviceDir, "types"),
		filepath.Join(serviceDir, "dsl"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0775); err != nil {
			return err
		}
	}

	return nil
}

func (g *ServiceGenerator) generateMainFile(serviceDir string, graph *dsl.ServiceGraph) error {
	// Read template from file
	mainTemplate, err := os.ReadFile("/tmp/main_template.txt")
	if err != nil {
		return fmt.Errorf("failed to read main template: %w", err)
	}
	tmpl := string(mainTemplate)

	data := struct {
		ServiceName       string
		ServiceNamePascal string
		Nodes             []dsl.Node
		Events            dsl.Events
	}{
		ServiceName:       graph.Metadata.Service,
		ServiceNamePascal: strings.Title(graph.Metadata.Service),
		Nodes:             graph.Nodes,
		Events:            graph.Events,
	}

	return g.executeTemplate(tmpl, data, filepath.Join(serviceDir, "main.go"))
}

func (g *ServiceGenerator) generateHandlers(serviceDir string, graph *dsl.ServiceGraph) error {
	// Read the corrected template from file
	tmplData, err := os.ReadFile("/home/hardik/Downloads/itsm-platform-mps-final/itsm-platform/cmd/codegen/templates/handlers_template.txt")
	if err != nil {
		return err
	}
	tmpl := string(tmplData)

	data := struct {
		ServiceName       string
		ServiceNamePascal string
		Nodes             []dsl.Node
	}{
		ServiceName:       graph.Metadata.Service,
		ServiceNamePascal: strings.Title(graph.Metadata.Service),
		Nodes:             graph.Nodes,
	}

	return g.executeTemplate(tmpl, data, filepath.Join(serviceDir, "handlers", "handlers.go"))
}

func (g *ServiceGenerator) generateTypes(serviceDir string, graph *dsl.ServiceGraph) error {
	tmpl := `package handlers

// Request types for NATS communication

type CreateRequest struct {
	TenantID string                 ` + "`" + `json:"tenant_id"` + "`" + `
	Data     map[string]interface{} ` + "`" + `json:"data"` + "`" + `
}

type UpdateRequest struct {
	TenantID string                 ` + "`" + `json:"tenant_id"` + "`" + `
	ID       string                 ` + "`" + `json:"id"` + "`" + `
	Data     map[string]interface{} ` + "`" + `json:"data"` + "`" + `
}

type DeleteRequest struct {
	TenantID string ` + "`" + `json:"tenant_id"` + "`" + `
	ID       string ` + "`" + `json:"id"` + "`" + `
}

type GetRequest struct {
	TenantID string   ` + "`" + `json:"tenant_id"` + "`" + `
	ID       string   ` + "`" + `json:"id"` + "`" + `
	Include  []string ` + "`" + `json:"include,omitempty"` + "`" + ` // For loading relations
}

type QueryRequest struct {
	TenantID string      ` + "`" + `json:"tenant_id"` + "`" + `
	Query    interface{} ` + "`" + `json:"query"` + "`" + `
	Include  []string    ` + "`" + `json:"include,omitempty"` + "`" + ` // For loading relations
}

// Event types for NATS publishing
type EntityEvent struct {
	TenantID  string                 ` + "`" + `json:"tenant_id"` + "`" + `
	EventType string                 ` + "`" + `json:"event_type"` + "`" + `
	Entity    string                 ` + "`" + `json:"entity"` + "`" + `
	EntityID  string                 ` + "`" + `json:"entity_id"` + "`" + `
	Data      map[string]interface{} ` + "`" + `json:"data"` + "`" + `
	Timestamp int64                  ` + "`" + `json:"timestamp"` + "`" + `
}

{{range $node := .Nodes}}
// {{$node.Name}} entity types
{{range $prop := $node.Properties}}{{if eq $prop.Type "enum"}}
type {{$node.Name}}{{$prop.Name | title}} string

const (
{{range $val := $prop.Values}}	{{$node.Name}}{{$prop.Name | title}}{{$val | title}} {{$node.Name}}{{$prop.Name | title}} = "{{$val}}"
{{end}}
)
{{end}}{{end}}
{{end}}`

	data := struct {
		Nodes []dsl.Node
	}{
		Nodes: graph.Nodes,
	}

	return g.executeTemplate(tmpl, data, filepath.Join(serviceDir, "types", "types.go"))
}

func (g *ServiceGenerator) generateDockerfile(serviceDir, serviceName string) error {
	tmpl := `# Dockerfile for {{.ServiceName}} service
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files from parent directory
COPY ../../go.mod ../../go.sum ./

RUN go mod download

# Copy source code
COPY . .

# Build the service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o {{.ServiceName}}-service .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

RUN addgroup -g 1000 -S service && \
    adduser -u 1000 -S service -G service

WORKDIR /app

COPY --from=builder /app/{{.ServiceName}}-service .
COPY --from=builder /app/dsl ./dsl

RUN chown -R service:service /app

USER service

CMD ["./{{.ServiceName}}-service"]`

	data := struct {
		ServiceName string
	}{
		ServiceName: serviceName,
	}

	return g.executeTemplate(tmpl, data, filepath.Join(serviceDir, "Dockerfile"))
}

func (g *ServiceGenerator) copyDSLFile(sourceDSL, serviceDir string) error {
	data, err := os.ReadFile(sourceDSL)
	if err != nil {
		return err
	}

	dslFile := filepath.Join(serviceDir, "dsl", "service.json")
	err = os.WriteFile(dslFile, data, 0644)
	if err != nil {
		return err
	}

	// Ensure proper permissions
	return os.Chmod(dslFile, 0664)
}

func (g *ServiceGenerator) generateGoMod(serviceDir string, graph *dsl.ServiceGraph) error {
	tmpl := `module {{.ServiceName}}-service

go 1.25

require (
	github.com/nats-io/nats.go v1.31.0
	itsm-platform v0.1.0
)

replace itsm-platform => ../../
`

	data := struct {
		ServiceName string
	}{
		ServiceName: graph.Metadata.Service,
	}

	return g.executeTemplate(tmpl, data, filepath.Join(serviceDir, "go.mod"))
}

func (g *ServiceGenerator) generateReadme(serviceDir string, graph *dsl.ServiceGraph) error {
	tmpl := `# {{.ServiceNamePascal}} Service

Generated service from DSL definition.

## Overview
{{.ServiceName}} service provides the following entities:
{{range .Nodes}}
### {{.Name}}
Table: {{.Table}}

Properties:
{{range .Properties}}- {{.Name}}: {{.Type}}{{if .Primary}} (Primary Key){{end}}{{if .Required}} (Required){{end}}
{{end}}
{{if .Relations}}
Relations:
{{range .Relations}}- {{.Name}}: {{.Type}} -> {{.TargetService}}.{{.TargetNode}}
{{end}}
{{end}}
{{if .Hooks.PreCreate.Enabled}}
Business Logic Hooks:
- Pre-create validations: {{len .Hooks.PreCreate.Validations}} rules
- Post-create actions: {{len .Hooks.PostCreate.Actions}} actions  
- Pre-update rules: {{len .Hooks.PreUpdate.Rules}} rules
- Post-update triggers: {{len .Hooks.PostUpdate.Triggers}} triggers
{{end}}
{{end}}

## Event Configuration
{{if .Events.Stream}}Stream: {{.Events.Stream}}{{end}}
{{if .Events.Publish}}
Published Events:
{{range .Events.Publish}}- {{.Event}}: {{.Subject}}
{{end}}
{{end}}
{{if .Events.Subscribe}}
Subscribed Events:
{{range .Events.Subscribe}}- {{.Subject}} -> {{.Handler}}
{{end}}
{{end}}

## Usage

### Build
` + "```bash" + `
go build
` + "```" + `

### Run
` + "```bash" + `
NATS_URL=nats://localhost:4222 ./{{.ServiceName}}-service
` + "```" + `

### Docker
` + "```bash" + `
docker build -t {{.ServiceName}}-service .
docker run -e NATS_URL=nats://nats:4222 {{.ServiceName}}-service
` + "```" + `

## API

All operations are available via NATS subjects:

{{range .Nodes}}### {{.Name}} Operations
- Create: {{$.ServiceName}}.{tenant_id}.{{.Name}}.create
- Update: {{$.ServiceName}}.{tenant_id}.{{.Name}}.update  
- Delete: {{$.ServiceName}}.{tenant_id}.{{.Name}}.delete
- Get: {{$.ServiceName}}.{tenant_id}.{{.Name}}.get
- Query: {{$.ServiceName}}.{tenant_id}.{{.Name}}.query

{{end}}## Development

This service was generated from the DSL definition in ` + "`dsl/service.json`" + `.
Modify the business logic in the handlers to customize behavior.
`

	data := struct {
		ServiceName       string
		ServiceNamePascal string
		Nodes             []dsl.Node
		Events            dsl.Events
	}{
		ServiceName:       graph.Metadata.Service,
		ServiceNamePascal: strings.Title(graph.Metadata.Service),
		Nodes:             graph.Nodes,
		Events:            graph.Events,
	}

	return g.executeTemplate(tmpl, data, filepath.Join(serviceDir, "README.md"))
}

func (g *ServiceGenerator) executeTemplate(tmplText string, data interface{}, outputPath string) error {
	funcMap := template.FuncMap{
		"title": strings.Title,
		"slice": func() []string { return make([]string, 0) },
		"append": func(slice []string, item string) []string {
			return append(slice, item)
		},
		"uniq": func(slice []string) []string {
			seen := make(map[string]bool)
			result := make([]string, 0)
			for _, item := range slice {
				if !seen[item] {
					seen[item] = true
					result = append(result, item)
				}
			}
			return result
		},
	}

	tmpl, err := template.New("generator").Funcs(funcMap).Parse(tmplText)
	if err != nil {
		return err
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	err = tmpl.Execute(file, data)
	if err != nil {
		return err
	}

	// Set proper file permissions (read/write for owner, read for group/others)
	return os.Chmod(outputPath, 0664)
}
