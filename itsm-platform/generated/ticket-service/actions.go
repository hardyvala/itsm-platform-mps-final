// =============================================================================
// ACTION HANDLERS - IMPLEMENT YOUR BUSINESS LOGIC HERE
// Generated from: ticket/service.json
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

// Actions for Ticket

// action_notify_customer - Called after ticket.Ticket is created
// TODO: Implement your business logic
func action_notify_customer(ctx context.Context, entity dal.Entity) error {
	log.Printf("ACTION [notify_customer]: entity_id=%v", entity["id"])
	// TODO: Implement notify_customer
	return nil
}

// action_auto_assign - Called after ticket.Ticket is created
// TODO: Implement your business logic
func action_auto_assign(ctx context.Context, entity dal.Entity) error {
	log.Printf("ACTION [auto_assign]: entity_id=%v", entity["id"])
	// TODO: Implement auto_assign
	return nil
}

// action_calculate_sla - Called after ticket.Ticket is created
// TODO: Implement your business logic
func action_calculate_sla(ctx context.Context, entity dal.Entity) error {
	log.Printf("ACTION [calculate_sla]: entity_id=%v", entity["id"])
	// TODO: Implement calculate_sla
	return nil
}
// Actions for Comment

// action_update_ticket_timestamp - Called after ticket.Comment is created
// TODO: Implement your business logic
func action_update_ticket_timestamp(ctx context.Context, entity dal.Entity) error {
	log.Printf("ACTION [update_ticket_timestamp]: entity_id=%v", entity["id"])
	// TODO: Implement update_ticket_timestamp
	return nil
}

// action_notify_watchers - Called after ticket.Comment is created
// TODO: Implement your business logic
func action_notify_watchers(ctx context.Context, entity dal.Entity) error {
	log.Printf("ACTION [notify_watchers]: entity_id=%v", entity["id"])
	// TODO: Implement notify_watchers
	return nil
}

// =============================================================================
// POST-UPDATE ACTIONS
// =============================================================================


// =============================================================================
// POST-DELETE ACTIONS
// =============================================================================


func action_cleanup_attachments(ctx context.Context, entity dal.Entity) error {
	log.Printf("ACTION [cleanup_attachments]: entity_id=%v", entity["id"])
	// TODO: Implement cleanup_attachments
	return nil
}

// =============================================================================
// TRIGGERS (on field change)
// =============================================================================


// trigger_notify_status_change - Called when status changes on Ticket
func trigger_notify_status_change(ctx context.Context, old, new dal.Entity, field string) error {
	log.Printf("TRIGGER [notify_status_change]: %s changed from %v to %v", 
		field, old[field], new[field])
	// TODO: Implement notify_status_change
	return nil
}

// trigger_notify_assignment - Called when assigned_to changes on Ticket
func trigger_notify_assignment(ctx context.Context, old, new dal.Entity, field string) error {
	log.Printf("TRIGGER [notify_assignment]: %s changed from %v to %v", 
		field, old[field], new[field])
	// TODO: Implement notify_assignment
	return nil
}

// =============================================================================
// PRE-DELETE CHECKS
// =============================================================================

