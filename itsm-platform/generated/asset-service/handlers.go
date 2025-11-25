// =============================================================================
// EVENT HANDLERS - FOR CROSS-SERVICE EVENTS
// Generated from: asset/service.json
// =============================================================================

package main

import (
	"context"
	"log"

	"itsm-platform/sdk/nats"
)

// =============================================================================
// SUBSCRIBE HANDLERS (from other services)
// =============================================================================


// on_customer_deleted - Handles events from: customer.*.customer.deleted
func on_customer_deleted(ctx context.Context, event nats.Event) error {
	log.Printf("EVENT [on_customer_deleted]: received from %s, tenant=%s", 
		event.Subject, event.TenantID)
	
	// TODO: Implement handler for customer.*.customer.deleted
	// event.Data contains the entity data
	
	return nil
}



// =============================================================================
// REGISTER HANDLERS (call this in main if needed)
// =============================================================================

func registerEventHandlers(em *nats.EventManager) {
	em.RegisterHandler("customer.*.customer.deleted", on_customer_deleted)
}
