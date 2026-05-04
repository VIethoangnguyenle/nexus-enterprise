// Package domain provides tenant provisioning business logic.
package domain

import (
	"context"
	"fmt"
)

// TenantProvisioner extends Store with schema provisioning.
type TenantProvisioner interface {
	ProvisionSchema(ctx context.Context, tenantID string) (string, error)
}

// ProvisionTenantSchema creates an isolated PostgreSQL schema for a tenant
// by calling the provision_tenant_schema() SQL function.
// Returns the created schema name.
func (s *Service) ProvisionTenantSchema(ctx context.Context, tenantID string) (string, error) {
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
