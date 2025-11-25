package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"itsm-platform/sdk/dal"
	"itsm-platform/sdk/registry"
)

// SetupRoutes configures all HTTP routes for a service based on DSL
func SetupRoutes(mux *http.ServeMux, svc *registry.Service) {
	// Auto-generate CRUD endpoints for all entities in DSL
	for _, node := range svc.Graph.Nodes {
		entityName := node.Name
		basePath := fmt.Sprintf("/api/v1/%s", node.Table)

		mux.HandleFunc(basePath, ListCreate(svc, entityName))
		mux.HandleFunc(basePath+"/", GetUpdateDelete(svc, entityName))
	}

	// Health check
	mux.HandleFunc("/health", Health(svc))

	// Tenant schema management
	mux.HandleFunc("/api/v1/tenants/schema", TenantSchema(svc))
}

// ListCreate handles GET (list) and POST (create)
func ListCreate(svc *registry.Service, entityName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx := r.Context()
		tenantID := r.Header.Get("X-Tenant-ID")

		if tenantID == "" {
			writeError(w, http.StatusBadRequest, "X-Tenant-ID header required")
			return
		}

		switch r.Method {
		case http.MethodGet:
			opts := parseListOptions(r)
			entities, total, err := svc.List(ctx, entityName, tenantID, opts)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data":   entities,
				"total":  total,
				"limit":  opts.Limit,
				"offset": opts.Offset,
			})

		case http.MethodPost:
			var data dal.Entity
			if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
				writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
				return
			}
			entity, err := svc.Create(ctx, entityName, tenantID, data)
			if err != nil {
				writeError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, entity)

		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}

// GetUpdateDelete handles GET, PUT/PATCH, DELETE for single entity
func GetUpdateDelete(svc *registry.Service, entityName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx := r.Context()
		tenantID := r.Header.Get("X-Tenant-ID")

		if tenantID == "" {
			writeError(w, http.StatusBadRequest, "X-Tenant-ID header required")
			return
		}

		// Extract ID from path
		id := extractID(r.URL.Path)
		if id == "" {
			writeError(w, http.StatusBadRequest, "Invalid ID in path")
			return
		}

		switch r.Method {
		case http.MethodGet:
			entity, err := svc.GetByID(ctx, entityName, tenantID, id)
			if err != nil {
				writeError(w, http.StatusNotFound, "Entity not found")
				return
			}
			writeJSON(w, http.StatusOK, entity)

		case http.MethodPut, http.MethodPatch:
			var data dal.Entity
			if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
				writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
				return
			}
			entity, err := svc.Update(ctx, entityName, tenantID, id, data)
			if err != nil {
				writeError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, entity)

		case http.MethodDelete:
			if err := svc.Delete(ctx, entityName, tenantID, id); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}

// Health check endpoint
func Health(svc *registry.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"service": svc.Graph.Metadata.Service,
		})
	}
}

// TenantSchema creates schema for a new tenant
func TenantSchema(svc *registry.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req struct {
			TenantID string `json:"tenant_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}

		if err := svc.CreateTenantSchema(r.Context(), req.TenantID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{
			"message":   "Schema created",
			"tenant_id": req.TenantID,
		})
	}
}

// Helper functions

func parseListOptions(r *http.Request) dal.ListOptions {
	opts := dal.DefaultListOptions()

	// Parse pagination
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 && l <= 100 {
			opts.Limit = l
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			opts.Offset = o
		}
	}

	// Parse ordering
	if orderBy := r.URL.Query().Get("order_by"); orderBy != "" {
		opts.OrderBy = orderBy
	}
	if orderDir := r.URL.Query().Get("order_dir"); orderDir != "" {
		if orderDir == "asc" || orderDir == "desc" {
			opts.OrderDir = strings.ToUpper(orderDir)
		}
	}

	// Parse filters (query params except pagination)
	for key, values := range r.URL.Query() {
		if key == "limit" || key == "offset" || key == "order_by" || key == "order_dir" {
			continue
		}
		if len(values) > 0 {
			opts.Filters[key] = values[0]
		}
	}

	return opts
}

func extractID(path string) string {
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
