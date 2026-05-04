// Package domain provides approval workflow business logic.
// It orchestrates template matching, approver resolution via NGAC,
// step advancement, and audit logging through the store and policy layers.
package domain

import (
	"context"
)

// PolicyClient abstracts the Policy Service calls needed by the approval domain.
// The concrete implementation will be wired in cmd/main.go.
type PolicyClient interface {
	// ResolveAccessibleScopes returns the leaf OA IDs a user can access for an operation.
	ResolveAccessibleScopes(ctx context.Context, userNodeID, operation string) ([]string, error)
	// CheckAccess verifies the user has the given operation on the target object.
	// Returns true if allowed, false if denied.
	CheckAccess(ctx context.Context, userNodeID, objectNodeID, operation string) (bool, error)
}

// Store defines the data access methods the domain service needs.
type Store interface {
	TemplateStore
	RequestStore
	AuditStore
}

// TemplateStore defines template CRUD operations.
type TemplateStore interface {
	InsertTemplate(ctx context.Context, t *Template) error
	GetTemplate(ctx context.Context, id string) (*Template, error)
	ListTemplates(ctx context.Context, entityType string, activeOnly bool) ([]*Template, error)
	UpdateTemplate(ctx context.Context, t *Template) error
}

// RequestStore defines approval request and assignment operations.
type RequestStore interface {
	InsertRequest(ctx context.Context, r *Request) error
	GetRequest(ctx context.Context, id string) (*Request, error)
	InsertAssignments(ctx context.Context, assignments []*AssignmentRecord) error
	GetAssignment(ctx context.Context, requestID, userNodeID string) (*AssignmentRecord, error)
	UpdateAssignmentStatus(ctx context.Context, id, status, comment string) error
	CountApprovedForStep(ctx context.Context, requestID string, stepOrder int) (int, error)
	SkipRemainingAssignments(ctx context.Context, requestID string, stepOrder int) error
	SkipAllPendingAssignments(ctx context.Context, requestID string) error
	AdvanceStep(ctx context.Context, requestID string, nextStep int) error
	CompleteRequest(ctx context.Context, requestID, status string) error

	// Query tabs
	ListPending(ctx context.Context, userNodeID string) ([]*RequestWithAssignment, error)
	ListHistory(ctx context.Context, userNodeID, cursor string, limit int) ([]*RequestWithAssignment, string, error)
	ListMyRequests(ctx context.Context, userNodeID, cursor string, limit int) ([]*Request, string, error)
	ListByScopes(ctx context.Context, scopeOAIDs []string, cursor string, limit int) ([]*Request, string, error)

	// Batch
	BatchApproveAssignments(ctx context.Context, userNodeID string, requestIDs []string, comment string) ([]string, error)
}

// AuditStore defines audit log operations.
type AuditStore interface {
	InsertAuditEntry(ctx context.Context, entry *AuditEntry) error
	ListAuditEntries(ctx context.Context, requestID string) ([]*AuditEntry, error)
}

// Service implements approval workflow business logic.
type Service struct {
	store  Store
	policy PolicyClient
}

// NewService creates an approval domain service with all required dependencies.
func NewService(s Store, pc PolicyClient) *Service {
	return &Service{store: s, policy: pc}
}
