-- Initial database setup for ITSM Platform

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create system schema for DAL metadata
CREATE SCHEMA IF NOT EXISTS dal_system;

-- Service registry table
CREATE TABLE IF NOT EXISTS dal_system.service_registry (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_name VARCHAR(100) UNIQUE NOT NULL,
    dsl_definition JSONB NOT NULL,
    version VARCHAR(20),
    registered_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Tenant registry
CREATE TABLE IF NOT EXISTS dal_system.tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(100) UNIQUE NOT NULL,
    tenant_name VARCHAR(255),
    status VARCHAR(50) DEFAULT 'active',
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Migration history
CREATE TABLE IF NOT EXISTS dal_system.migration_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_name VARCHAR(100),
    tenant_id VARCHAR(100),
    migration_type VARCHAR(50),
    migration_detail TEXT,
    executed_at TIMESTAMPTZ DEFAULT NOW(),
    success BOOLEAN DEFAULT true,
    error_message TEXT
);

-- Create indexes
CREATE INDEX idx_service_registry_name ON dal_system.service_registry(service_name);
CREATE INDEX idx_tenants_tenant_id ON dal_system.tenants(tenant_id);
CREATE INDEX idx_migration_history_service ON dal_system.migration_history(service_name, tenant_id);

-- Create default tenant for development
INSERT INTO dal_system.tenants (tenant_id, tenant_name, metadata)
VALUES ('default', 'Default Tenant', '{"type": "development"}')
ON CONFLICT (tenant_id) DO NOTHING;

-- Grant permissions
GRANT ALL PRIVILEGES ON SCHEMA dal_system TO itsm_user;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA dal_system TO itsm_user;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA dal_system TO itsm_user;