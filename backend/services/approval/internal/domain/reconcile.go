package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"ngac-platform/pkg/httputil"
)

// TenantSchema names an organization's approval schema.
type TenantSchema struct {
	TenantID string
	Schema   string
}

// ReconcileStore is what the sweep over requests already in progress needs.
type ReconcileStore interface {
	// ListOrganizationSchemas returns the active approval schemas of
	// organization workspaces (a personal workspace's approvals are kept as
	// they are, hidden).
	ListOrganizationSchemas(ctx context.Context) ([]TenantSchema, error)
	// ListPendingRequestIDs returns the ids of the tenant's pending requests.
	ListPendingRequestIDs(ctx context.Context) ([]string, error)
}

// ReconcileSoleApproverSteps unsticks requests that were already waiting, when
// skipping was introduced, on a step only their requester could decide: such a
// step can never be decided, because the requester may not act and nobody else
// can. Each pending request of every organization is looked at once, under its
// row lock, with the same rule a step uses when it starts (startStep), so the
// sweep is idempotent and safe to run on every start of the service. A request
// on whose current step anyone has already acted is left alone.
//
// It returns how many requests had a step skipped. Failures on one tenant or
// request are collected and the sweep goes on with the others.
func (s *Service) ReconcileSoleApproverSteps(ctx context.Context) (int, error) {
	tenants, err := s.store.ListOrganizationSchemas(ctx)
	if err != nil {
		return 0, fmt.Errorf("list tenant schemas: %w", err)
	}
	skipped := 0
	var errs []error
	for _, t := range tenants {
		tctx := httputil.WithTenantSchema(ctx, t.Schema)
		ids, err := s.store.ListPendingRequestIDs(tctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("tenant %s: list pending: %w", t.TenantID, err))
			continue
		}
		for _, id := range ids {
			ok, err := s.unstickRequest(tctx, t.TenantID, id)
			if err != nil {
				errs = append(errs, fmt.Errorf("tenant %s request %s: %w", t.TenantID, id, err))
				continue
			}
			if ok {
				skipped++
			}
		}
	}
	return skipped, errors.Join(errs...)
}

// unstickRequest skips the request's current step if only its requester could
// decide it and nobody has acted on it, and reports whether it did.
func (s *Service) unstickRequest(ctx context.Context, tenantID, requestID string) (bool, error) {
	skipped := false
	err := s.store.InTx(ctx, func(ctx context.Context) error {
		req, err := s.lockPending(ctx, requestID)
		if errors.Is(err, ErrRequestCompleted) {
			return nil
		}
		if err != nil {
			return err
		}
		var tmpl Template
		if err := json.Unmarshal([]byte(req.TemplateSnapshot), &tmpl); err != nil {
			return fmt.Errorf("unmarshal snapshot: %w", err)
		}
		step := stepAt(tmpl.Steps, req.CurrentStep)
		if step == nil {
			return nil
		}
		rows, err := s.store.ListAssignments(ctx, requestID)
		if err != nil {
			return fmt.Errorf("list assignments: %w", err)
		}
		for _, a := range rows {
			if a.StepOrder == req.CurrentStep && a.Status != "pending" {
				return nil // someone has decided on this step: theirs to finish
			}
		}
		built, err := s.stepAssignments(ctx, req, step, tenantID)
		if err != nil {
			// A step that resolves to nobody now is not one to skip.
			if errors.Is(err, ErrInvalidInput) {
				slog.Warn("approval reconcile: current step resolves to nobody", "request_id", requestID, "error", err)
				return nil
			}
			return err
		}
		sole, err := s.requesterIsSoleApprover(ctx, built[0], req.CreatedBy)
		if err != nil || !sole {
			return err
		}
		if err := s.store.SkipRemainingAssignments(ctx, requestID, req.CurrentStep); err != nil {
			return fmt.Errorf("skip remaining: %w", err)
		}
		skipped = true
		return s.skipStep(ctx, req, tmpl.Steps, req.CurrentStep, tenantID)
	})
	if err != nil {
		return false, err
	}
	return skipped, nil
}
