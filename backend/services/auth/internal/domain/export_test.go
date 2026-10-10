package domain

import "context"

// InitTenantNGAC exposes the tenant graph initialisation to the black-box tests.
func (s *Service) InitTenantNGAC(ctx context.Context, tenantID, pcNodeID, ownersUAID, membersUAID string) error {
	return s.initTenantNGAC(ctx, tenantID, pcNodeID, ownersUAID, membersUAID)
}
