package hooks

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"itsm-platform/sdk/dal"
	"itsm-platform/sdk/parser"
)

// Executor runs DSL-defined validations and dispatches actions
type Executor struct {
	node            *parser.Node
	actionHandlers  map[string]ActionHandler
	triggerHandlers map[string]TriggerHandler
	checkHandlers   map[string]CheckHandler
}

// ActionHandler handles post-operation actions
type ActionHandler func(ctx context.Context, entity dal.Entity) error

// TriggerHandler handles field-change triggers
type TriggerHandler func(ctx context.Context, old, new dal.Entity, field string) error

// CheckHandler handles pre-delete checks
type CheckHandler func(ctx context.Context, entity dal.Entity) error

// NewExecutor creates a hook executor from DSL node
func NewExecutor(node *parser.Node) *Executor {
	return &Executor{
		node:            node,
		actionHandlers:  make(map[string]ActionHandler),
		triggerHandlers: make(map[string]TriggerHandler),
		checkHandlers:   make(map[string]CheckHandler),
	}
}

// RegisterAction registers a handler for an action name
func (e *Executor) RegisterAction(name string, handler ActionHandler) {
	e.actionHandlers[name] = handler
}

// RegisterTrigger registers a handler for a trigger action
func (e *Executor) RegisterTrigger(name string, handler TriggerHandler) {
	e.triggerHandlers[name] = handler
}

// RegisterCheck registers a handler for a pre-delete check
func (e *Executor) RegisterCheck(name string, handler CheckHandler) {
	e.checkHandlers[name] = handler
}

// PreCreate runs pre-create validations from DSL
func (e *Executor) PreCreate(ctx context.Context, entity dal.Entity) error {
	if !e.node.Hooks.PreCreate.Enabled {
		return nil
	}

	// Run DSL validations
	for _, v := range e.node.Hooks.PreCreate.Validations {
		if err := e.runValidation(entity, v); err != nil {
			return err
		}
	}

	return nil
}

// PostCreate runs post-create actions from DSL
func (e *Executor) PostCreate(ctx context.Context, entity dal.Entity) error {
	if !e.node.Hooks.PostCreate.Enabled {
		return nil
	}

	// Run DSL actions
	for _, action := range e.node.Hooks.PostCreate.Actions {
		if handler, ok := e.actionHandlers[action]; ok {
			if err := handler(ctx, entity); err != nil {
				return fmt.Errorf("action %s failed: %w", action, err)
			}
		}
	}

	return nil
}

// PreUpdate runs pre-update validations and rules from DSL
func (e *Executor) PreUpdate(ctx context.Context, old, new dal.Entity) error {
	if !e.node.Hooks.PreUpdate.Enabled {
		return nil
	}

	// Run DSL validations
	for _, v := range e.node.Hooks.PreUpdate.Validations {
		if err := e.runValidation(new, v); err != nil {
			return err
		}
	}

	// Run DSL rules
	for _, rule := range e.node.Hooks.PreUpdate.Rules {
		if e.evaluateCondition(rule.Condition, old, new) {
			if rule.Action == "reject" {
				return fmt.Errorf(rule.Message)
			}
		}
	}

	return nil
}

// PostUpdate runs post-update triggers from DSL
func (e *Executor) PostUpdate(ctx context.Context, old, new dal.Entity) error {
	if !e.node.Hooks.PostUpdate.Enabled {
		return nil
	}

	// Run DSL triggers on field changes
	for _, trigger := range e.node.Hooks.PostUpdate.Triggers {
		if e.fieldChanged(old, new, trigger.OnFieldChange) {
			if handler, ok := e.triggerHandlers[trigger.Action]; ok {
				if err := handler(ctx, old, new, trigger.OnFieldChange); err != nil {
					return fmt.Errorf("trigger %s failed: %w", trigger.Action, err)
				}
			}
		}
	}

	// Run DSL actions
	for _, action := range e.node.Hooks.PostUpdate.Actions {
		if handler, ok := e.actionHandlers[action]; ok {
			if err := handler(ctx, new); err != nil {
				return fmt.Errorf("action %s failed: %w", action, err)
			}
		}
	}

	return nil
}

// PreDelete runs pre-delete checks from DSL
func (e *Executor) PreDelete(ctx context.Context, entity dal.Entity) error {
	if !e.node.Hooks.PreDelete.Enabled {
		return nil
	}

	// Run DSL checks
	for _, check := range e.node.Hooks.PreDelete.Checks {
		if handler, ok := e.checkHandlers[check]; ok {
			if err := handler(ctx, entity); err != nil {
				return fmt.Errorf("check %s failed: %w", check, err)
			}
		}
	}

	return nil
}

// PostDelete runs post-delete actions from DSL
func (e *Executor) PostDelete(ctx context.Context, entity dal.Entity) error {
	if !e.node.Hooks.PostDelete.Enabled {
		return nil
	}

	for _, action := range e.node.Hooks.PostDelete.Actions {
		if handler, ok := e.actionHandlers[action]; ok {
			if err := handler(ctx, entity); err != nil {
				return fmt.Errorf("action %s failed: %w", action, err)
			}
		}
	}

	return nil
}

// runValidation executes a single validation
func (e *Executor) runValidation(entity dal.Entity, v parser.Validation) error {
	value, exists := entity[v.Field]

	switch v.Rule {
	case "required":
		if !exists || value == nil || value == "" {
			return fmt.Errorf(v.Message)
		}

	case "min_length":
		if str, ok := value.(string); ok {
			minLen, _ := v.Value.(float64) // JSON numbers are float64
			if len(str) < int(minLen) {
				return fmt.Errorf(v.Message)
			}
		}

	case "max_length":
		if str, ok := value.(string); ok {
			maxLen, _ := v.Value.(float64)
			if len(str) > int(maxLen) {
				return fmt.Errorf(v.Message)
			}
		}

	case "email_format":
		if str, ok := value.(string); ok {
			emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
			if !emailRegex.MatchString(str) {
				return fmt.Errorf(v.Message)
			}
		}

	case "regex":
		if str, ok := value.(string); ok {
			pattern, _ := v.Value.(string)
			regex := regexp.MustCompile(pattern)
			if !regex.MatchString(str) {
				return fmt.Errorf(v.Message)
			}
		}

	case "in":
		if allowed, ok := v.Value.([]interface{}); ok {
			found := false
			for _, a := range allowed {
				if value == a {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf(v.Message)
			}
		}
	}

	return nil
}

// evaluateCondition evaluates a simple condition expression
// Supports: old.field == 'value', new.field != 'value', etc.
func (e *Executor) evaluateCondition(condition string, old, new dal.Entity) bool {
	// Parse condition like "old.status == 'closed' && new.status != 'closed'"
	// Simple implementation - production would use expression parser

	parts := strings.Split(condition, "&&")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !e.evaluateSingleCondition(part, old, new) {
			return false
		}
	}
	return true
}

func (e *Executor) evaluateSingleCondition(cond string, old, new dal.Entity) bool {
	// Parse "old.status == 'closed'" or "new.status != 'closed'"
	var entity dal.Entity
	var op, value string
	var field string

	if strings.HasPrefix(cond, "old.") {
		entity = old
		cond = strings.TrimPrefix(cond, "old.")
	} else if strings.HasPrefix(cond, "new.") {
		entity = new
		cond = strings.TrimPrefix(cond, "new.")
	} else {
		return false
	}

	if strings.Contains(cond, "==") {
		parts := strings.Split(cond, "==")
		field = strings.TrimSpace(parts[0])
		op = "=="
		value = strings.Trim(strings.TrimSpace(parts[1]), "'\"")
	} else if strings.Contains(cond, "!=") {
		parts := strings.Split(cond, "!=")
		field = strings.TrimSpace(parts[0])
		op = "!="
		value = strings.Trim(strings.TrimSpace(parts[1]), "'\"")
	}

	entityValue, _ := entity[field].(string)

	switch op {
	case "==":
		return entityValue == value
	case "!=":
		return entityValue != value
	}

	return false
}

// fieldChanged checks if a field value changed between old and new
func (e *Executor) fieldChanged(old, new dal.Entity, field string) bool {
	oldVal, _ := old[field]
	newVal, _ := new[field]
	return oldVal != newVal
}
