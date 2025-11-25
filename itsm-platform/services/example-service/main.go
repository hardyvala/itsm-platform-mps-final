package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/nats-io/nats.go"
	dalclient "itsm-platform/services/dal-service/client"
)

// Example service showing how to use the DAL service
type TicketService struct {
	nc  *nats.Conn
	dal *dalclient.Client
}

func main() {
	// Connect to NATS
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	// Create service
	service := &TicketService{
		nc:  nc,
		dal: dalclient.NewClient(nc, "ticket"),
	}

	// Register service DSL with DAL
	if err := service.registerWithDAL(); err != nil {
		log.Fatal(err)
	}

	// Example: Create a ticket
	ctx := context.Background()
	tenantID := "tenant_123"

	// Ensure tenant exists
	if err := service.dal.CreateTenant(tenantID); err != nil {
		log.Printf("Tenant might already exist: %v", err)
	}

	// Create a ticket
	ticket := map[string]interface{}{
		"title":       "Server is down",
		"description": "Production server not responding",
		"status":      "open",
		"priority":    "critical",
		"category":    "infrastructure",
	}

	created, err := service.dal.Create(ctx, tenantID, "ticket", ticket)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created ticket: %v\n", created)

	// Query tickets
	query := dalclient.NewQueryBuilder().
		Where("status", "eq", "open").
		Where("priority", "in", []string{"high", "critical"}).
		OrderBy("created_at", true).
		Limit(10).
		Build()

	results, err := service.dal.Query(ctx, tenantID, "ticket", query)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d tickets\n", results.Total)

	// Subscribe to ticket events
	service.nc.Subscribe("ticket.*.ticket.created", func(msg *nats.Msg) {
		var event map[string]interface{}
		json.Unmarshal(msg.Data, &event)
		fmt.Printf("New ticket created: %v\n", event)
	})

	// Keep service running
	select {}
}

func (s *TicketService) registerWithDAL() error {
	// Define service DSL
	dsl := map[string]interface{}{
		"version": "1.0",
		"kind":    "ServiceGraph",
		"metadata": map[string]interface{}{
			"service": "ticket",
			"version": "1.0.0",
		},
		"nodes": []map[string]interface{}{
			{
				"name":  "ticket",
				"table": "tickets",
				"properties": []map[string]interface{}{
					{
						"name":       "title",
						"type":       "string",
						"required":   true,
						"max_length": 255,
					},
					{
						"name":     "description",
						"type":     "text",
						"required": false,
					},
					{
						"name":     "status",
						"type":     "enum",
						"values":   []string{"open", "in_progress", "resolved", "closed"},
						"required": true,
						"indexed":  true,
					},
					{
						"name":     "priority",
						"type":     "enum",
						"values":   []string{"low", "medium", "high", "critical"},
						"required": true,
						"indexed":  true,
					},
					{
						"name":    "category",
						"type":    "string",
						"indexed": true,
					},
					{
						"name":    "assignee_id",
						"type":    "uuid",
						"indexed": true,
					},
					{
						"name":    "reporter_id",
						"type":    "uuid",
						"indexed": true,
					},
					{
						"name": "resolved_at",
						"type": "timestamp",
					},
					{
						"name": "metadata",
						"type": "jsonb",
					},
				},
				"indexes": []map[string]interface{}{
					{
						"name":   "idx_status_priority",
						"fields": []string{"status", "priority"},
					},
					{
						"name":   "idx_category_status",
						"fields": []string{"category", "status"},
					},
				},
				"dal": map[string]interface{}{
					"soft_delete":     true,
					"optimistic_lock": true,
				},
			},
			{
				"name":  "comment",
				"table": "comments",
				"properties": []map[string]interface{}{
					{
						"name":     "ticket_id",
						"type":     "uuid",
						"required": true,
						"indexed":  true,
					},
					{
						"name":     "author_id",
						"type":     "uuid",
						"required": true,
						"indexed":  true,
					},
					{
						"name":     "content",
						"type":     "text",
						"required": true,
					},
					{
						"name": "is_internal",
						"type": "boolean",
					},
				},
				"indexes": []map[string]interface{}{
					{
						"name":   "idx_ticket_created",
						"fields": []string{"ticket_id", "created_at"},
					},
				},
				"dal": map[string]interface{}{
					"soft_delete": false,
				},
			},
		},
		"edges": []map[string]interface{}{
			{
				"name":      "comments",
				"from":      "ticket",
				"to":        "comment",
				"type":      "one_to_many",
				"local_key": "ticket_id",
			},
			{
				"name":      "assignee",
				"from":      "ticket",
				"to":        "user",
				"type":      "many_to_one",
				"local_key": "assignee_id",
				"external":  true,
				"service":   "user-service",
			},
		},
	}

	return s.dal.RegisterService("ticket", dsl)
}

// Example of loading DSL from file
func loadDSLFromFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var dsl map[string]interface{}
	if err := json.Unmarshal(data, &dsl); err != nil {
		return nil, err
	}

	return dsl, nil
}
