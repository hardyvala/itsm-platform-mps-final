// =============================================================================
// ACTION HANDLERS - IMPLEMENT YOUR BUSINESS LOGIC HERE
// Generated from: asset/service.json
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

// Actions for Asset

// action_log_asset_creation - Called after asset.Asset is created
// TODO: Implement your business logic
func action_log_asset_creation(ctx context.Context, entity dal.Entity) error {
	log.Printf("ACTION [log_asset_creation]: entity_id=%v", entity["id"])
	// TODO: Implement log_asset_creation
	return nil
}

// =============================================================================
// POST-UPDATE ACTIONS
// =============================================================================


// =============================================================================
// POST-DELETE ACTIONS
// =============================================================================


func action_archive_asset_history(ctx context.Context, entity dal.Entity) error {
	log.Printf("ACTION [archive_asset_history]: entity_id=%v", entity["id"])
	// TODO: Implement archive_asset_history
	return nil
}

// =============================================================================
// TRIGGERS (on field change)
// =============================================================================


// trigger_log_status_change - Called when status changes on Asset
func trigger_log_status_change(ctx context.Context, old, new dal.Entity, field string) error {
	log.Printf("TRIGGER [log_status_change]: %s changed from %v to %v", 
		field, old[field], new[field])
	// TODO: Implement log_status_change
	return nil
}

// trigger_notify_assignment_change - Called when assigned_to changes on Asset
func trigger_notify_assignment_change(ctx context.Context, old, new dal.Entity, field string) error {
	log.Printf("TRIGGER [notify_assignment_change]: %s changed from %v to %v", 
		field, old[field], new[field])
	// TODO: Implement notify_assignment_change
	return nil
}

// =============================================================================
// PRE-DELETE CHECKS
// =============================================================================


// check_not_assigned - Validates before Asset can be deleted
func check_not_assigned(ctx context.Context, entity dal.Entity) error {
	log.Printf("CHECK [not_assigned]: entity_id=%v", entity["id"])
	// TODO: Implement not_assigned - return error to prevent deletion
	return nil
}
