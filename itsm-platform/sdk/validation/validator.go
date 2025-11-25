package validation

import (
	"fmt"
	"log"
)

// Validator provides common validation functionality
type Validator struct{}

// ValidateField performs field validation based on rules
func (v *Validator) ValidateField(fieldName string, data map[string]interface{}, rule string, value interface{}, message string) error {
	fieldValue, exists := data[fieldName]
	if !exists && rule == "required" {
		return fmt.Errorf(message)
	}

	switch rule {
	case "required":
		if fieldValue == nil || fieldValue == "" {
			return fmt.Errorf(message)
		}
	case "min_length":
		if str, ok := fieldValue.(string); ok {
			minLen := int(value.(float64))
			if len(str) < minLen {
				return fmt.Errorf(message)
			}
		}
	case "max_length":
		if str, ok := fieldValue.(string); ok {
			maxLen := int(value.(float64))
			if len(str) > maxLen {
				return fmt.Errorf(message)
			}
		}
	case "email_format":
		// Basic email validation (extend as needed)
		if str, ok := fieldValue.(string); ok {
			if !v.isValidEmail(str) {
				return fmt.Errorf(message)
			}
		}
	}
	return nil
}

// ExecuteAction provides common action execution functionality
func (v *Validator) ExecuteAction(action string, tenantID string, data map[string]interface{}) error {
	log.Printf("Executing action: %s for tenant: %s", action, tenantID)
	// Base implementation - services can override specific actions
	return nil
}

// isValidEmail performs basic email validation
func (v *Validator) isValidEmail(email string) bool {
	// Simple email validation - extend as needed
	return len(email) > 0 &&
		len(email) < 255 &&
		email != "" &&
		email != " "
}
