// Package domain provides tenant provisioning business logic.
package domain

import (
	"context"
	"errors"
	"fmt"
)

// TenantProvisioner extends Store with schema provisioning.
type TenantProvisioner interface {
	ProvisionSchema(ctx context.Context, tenantID string) (string, error)
}

// SchemaEnsurer creates a tenant's schema only if it has none.
type SchemaEnsurer interface {
	EnsureSchema(ctx context.Context, tenantID string) (string, error)
}

// EnsureTenantSchema is what an organization's first approval call runs: it
// returns the tenant's schema name, creating the schema when the tenant has none
// (a personal workspace that was just converted). A schema that is registered but
// not active is never created over or reactivated; ErrSchemaNotActive comes back.
// A personal workspace has no approvals and is refused.
func (s *Service) EnsureTenantSchema(ctx context.Context, tenantID string) (string, error) {
	if err := s.RequireOrganization(ctx, tenantID); err != nil {
		return "", err
	}
	if tenantID == "" {
		return "", ErrInvalidInput
	}
	ensurer, ok := s.store.(SchemaEnsurer)
	if !ok {
		return "", fmt.Errorf("store does not implement SchemaEnsurer")
	}
	schema, err := ensurer.EnsureSchema(ctx, tenantID)
	if err != nil {
		if errors.Is(err, ErrSchemaNotActive) {
			return "", err
		}
		return "", fmt.Errorf("ensure tenant schema: %w", err)
	}
	return schema, nil
}

// ProvisionTenantSchema creates an isolated PostgreSQL schema for a tenant
// by calling the provision_tenant_schema() SQL function.
// Returns the created schema name.
func (s *Service) ProvisionTenantSchema(ctx context.Context, tenantID string) (string, error) {
	// A personal workspace has no approvals, so it gets no approval schema.
	if err := s.RequireOrganization(ctx, tenantID); err != nil {
		return "", err
	}
	if tenantID == "" {
		return "", ErrInvalidInput
	}

	provisioner, ok := s.store.(TenantProvisioner)
	if !ok {
		return "", fmt.Errorf("store does not implement TenantProvisioner")
	}

	schema, err := provisioner.ProvisionSchema(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("provision tenant schema: %w", err)
	}
	return schema, nil
}
