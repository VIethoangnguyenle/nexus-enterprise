// Package domain provides approval workflow business logic.
// It orchestrates template matching, approver resolution via NGAC,
// step advancement, and audit logging through the store and policy layers.
package domain

import (
	"context"
	"time"
)

// PolicyClient abstracts the Policy Service calls needed by the approval domain.
// The concrete implementation will be wired in cmd/main.go.
type PolicyClient interface {
	// ResolveAccessibleScopes returns the leaf OA IDs a user can access for an operation.
	ResolveAccessibleScopes(ctx context.Context, userNodeID, operation string) ([]string, error)
	// CheckAccess verifies the user has the given operation on the target object.
	// Returns true if allowed, false if denied.
	CheckAccess(ctx context.Context, userNodeID, objectNodeID, operation string) (bool, error)
	// GetAncestors returns the ids of every node above the given one: for a
	// person, the roles and departments they belong to right now.
	GetAncestors(ctx context.Context, nodeID string) ([]string, error)
	// GetMembers returns the people (user nodes) under a role or department.
	GetMembers(ctx context.Context, uaNodeID string) ([]string, error)
}

// Store defines the data access methods the domain service needs.
type Store interface {
	TemplateStore
	RequestStore
	AuditStore
	DirectoryStore
	Transactor
}

// Transactor runs several store calls as one change.
type Transactor interface {
	// InTx runs fn with every store call made on the context it is given inside
	// one transaction: all of them take effect or none does. fn returning an
	// error rolls them all back.
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// DirectoryStore answers questions about who and what exists in a tenant, from
// the shared (non-tenant) tables.
type DirectoryStore interface {
	// FindNodeID returns the id of the node with this exact name and type, or ErrNotFound.
	FindNodeID(ctx context.Context, name, nodeType string) (string, error)
	// CanonicalApprover checks that an approver named by a template step exists
	// in this tenant and returns the key the step should store: a person's user
	// node, a role's UA, or a department's UA (a department id is turned into
	// its UA). Anything else is ErrInvalidInput.
	CanonicalApprover(ctx context.Context, tenantID, approverType, value string) (string, error)
}

// TemplateStore defines template CRUD operations.
type TemplateStore interface {
	InsertTemplate(ctx context.Context, t *Template) error
	GetTemplate(ctx context.Context, id string) (*Template, error)
	ListTemplates(ctx context.Context, entityType string, activeOnly bool) ([]*Template, error)
	// UpdateTemplate saves t only if its stored updated_at still equals expected,
	// and returns the new updated_at. A changed template is ErrStale.
	UpdateTemplate(ctx context.Context, t *Template, expected time.Time) (time.Time, error)
}

// RequestStore defines approval request and assignment operations.
type RequestStore interface {
	InsertRequest(ctx context.Context, r *Request) error
	// InsertRequestWithAssignments stores a request and its first assignments as
	// one change, so a request never exists with nobody to decide it.
	InsertRequestWithAssignments(ctx context.Context, r *Request, assignments []*AssignmentRecord) error
	GetRequest(ctx context.Context, id string) (*Request, error)
	InsertAssignments(ctx context.Context, assignments []*AssignmentRecord) error
	GetAssignment(ctx context.Context, requestID, userNodeID string) (*AssignmentRecord, error)
	// ListAssignments returns every assignment of a request, by step, then in
	// the order the approvers acted.
	ListAssignments(ctx context.Context, requestID string) ([]*AssignmentRecord, error)
	// HasAssignment reports whether any of the nodes (a person, or the roles
	// and departments they belong to) has an assignment of any status on any
	// step of the request, current or past.
	HasAssignment(ctx context.Context, requestID string, nodeIDs []string) (bool, error)
	// FindGroupAssignment returns the pending role or department row of a step
	// whose group is among groupNodeIDs, or ErrNotFound.
	FindGroupAssignment(ctx context.Context, requestID string, stepOrder int, groupNodeIDs []string) (*AssignmentRecord, error)
	// InsertActedAssignment records a person's own decision on a step they were
	// assigned through a group. A person already holding a row on the step is
	// ErrAlreadyExists.
	InsertActedAssignment(ctx context.Context, a *AssignmentRecord) error
	UpdateAssignmentStatus(ctx context.Context, id, status, comment string) error
	CountApprovedForStep(ctx context.Context, requestID string, stepOrder int) (int, error)
	SkipRemainingAssignments(ctx context.Context, requestID string, stepOrder int) error
	SkipAllPendingAssignments(ctx context.Context, requestID string) error
	// LockRequest takes the request's row lock until the surrounding InTx ends and
	// returns its status and current step as they stand under it. Missing is
	// ErrNotFound.
	LockRequest(ctx context.Context, requestID string) (status string, currentStep int, err error)
	AdvanceStep(ctx context.Context, requestID string, fromStep, nextStep int) (bool, error)
	CompleteRequest(ctx context.Context, requestID, status string) (bool, error)
	// ListPendingAssignees returns the user nodes still pending on a step.
	ListPendingAssignees(ctx context.Context, requestID string, stepOrder int) ([]string, error)

	// Query tabs
	// ListPending returns what waits on the user: their own rows and the group
	// rows of the roles and departments in groupNodeIDs, minus steps they have
	// already acted on.
	ListPending(ctx context.Context, userNodeID string, groupNodeIDs []string) ([]*RequestWithAssignment, error)
	ListHistory(ctx context.Context, userNodeID, cursor string, limit int) ([]*RequestWithAssignment, string, error)
	ListMyRequests(ctx context.Context, userNodeID, cursor string, limit int) ([]*Request, string, error)
	ListByScopes(ctx context.Context, scopeOAIDs []string, cursor string, limit int) ([]*Request, string, error)

	// Batch
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
