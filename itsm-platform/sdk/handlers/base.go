package handlers

import (
	"encoding/json"
	"log"
	"reflect"

	"github.com/nats-io/nats.go"
)

// Common request types for NATS communication
type CreateRequest struct {
	TenantID string                 `json:"tenant_id"`
	Data     map[string]interface{} `json:"data"`
}

type UpdateRequest struct {
	TenantID string                 `json:"tenant_id"`
	ID       string                 `json:"id"`
	Data     map[string]interface{} `json:"data"`
}

type DeleteRequest struct {
	TenantID string `json:"tenant_id"`
	ID       string `json:"id"`
}

type GetRequest struct {
	TenantID string   `json:"tenant_id"`
	ID       string   `json:"id"`
	Include  []string `json:"include,omitempty"` // For loading relations
}

type QueryRequest struct {
	TenantID string      `json:"tenant_id"`
	Query    interface{} `json:"query"`
	Include  []string    `json:"include,omitempty"` // For loading relations
}

// BaseHandlers provides common functionality for all service handlers
type BaseHandlers struct{}

// EvaluateCondition provides simple condition evaluation
func (b *BaseHandlers) EvaluateCondition(condition string, oldData, newData map[string]interface{}) bool {
	// Simple condition evaluation (extend as needed)
	// For now, just log the condition
	log.Printf("Evaluating condition: %s", condition)
	return false
}

// HasFieldChanged checks if a specific field has changed between old and new data
func (b *BaseHandlers) HasFieldChanged(fieldName string, oldData, newData map[string]interface{}) bool {
	oldVal := oldData[fieldName]
	newVal := newData[fieldName]
	return !reflect.DeepEqual(oldVal, newVal)
}

// ReplySuccess sends a successful response back through NATS
func (b *BaseHandlers) ReplySuccess(msg *nats.Msg, data interface{}) {
	response := map[string]interface{}{
		"success": true,
		"data":    data,
	}
	payload, _ := json.Marshal(response)
	msg.Respond(payload)
}

// ReplyError sends an error response back through NATS
func (b *BaseHandlers) ReplyError(msg *nats.Msg, err error) {
	response := map[string]interface{}{
		"success": false,
		"error":   err.Error(),
	}
	payload, _ := json.Marshal(response)
	msg.Respond(payload)
}
