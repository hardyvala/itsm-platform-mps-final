// =============================================================================
// ACTION HANDLERS - IMPLEMENT YOUR BUSINESS LOGIC HERE
// Generated from: customer/service.json
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

// Actions for Customer

// action_send_welcome_email - Called after customer.Customer is created
// TODO: Implement your business logic
func action_send_welcome_email(ctx context.Context, entity dal.Entity) error {
	log.Printf("ACTION [send_welcome_email]: entity_id=%v", entity["id"])
	// TODO: Implement send_welcome_email
	return nil
}

// =============================================================================
// POST-UPDATE ACTIONS
// =============================================================================


// =============================================================================
// POST-DELETE ACTIONS
// =============================================================================


func action_anonymize_data(ctx context.Context, entity dal.Entity) error {
	log.Printf("ACTION [anonymize_data]: entity_id=%v", entity["id"])
	// TODO: Implement anonymize_data
	return nil
}

// =============================================================================
// TRIGGERS (on field change)
// =============================================================================


// trigger_send_email_change_notification - Called when email changes on Customer
func trigger_send_email_change_notification(ctx context.Context, old, new dal.Entity, field string) error {
	log.Printf("TRIGGER [send_email_change_notification]: %s changed from %v to %v", 
		field, old[field], new[field])
	// TODO: Implement send_email_change_notification
	return nil
}

// =============================================================================
// PRE-DELETE CHECKS
// =============================================================================


// check_has_no_open_tickets - Validates before Customer can be deleted
func check_has_no_open_tickets(ctx context.Context, entity dal.Entity) error {
	log.Printf("CHECK [has_no_open_tickets]: entity_id=%v", entity["id"])
	// TODO: Implement has_no_open_tickets - return error to prevent deletion
	return nil
}
