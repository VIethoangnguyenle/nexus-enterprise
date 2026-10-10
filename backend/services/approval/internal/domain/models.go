package domain

import "time"

// FormField defines a single dynamic form field within a template.
// Templates use these to specify what data submitters must provide.
type FormField struct {
	Label       string `json:"label"`
	FieldType   string `json:"field_type"` // "text", "number", "currency", "date", "select", "textarea"
	Required    bool   `json:"required"`
	Options     string `json:"options,omitempty"` // comma-separated options for "select" type
	FieldOrder  int    `json:"field_order"`       // display order
	Placeholder string `json:"placeholder,omitempty"`
}

// Template represents an approval workflow template definition.
type Template struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	EntityType     string       `json:"entity_type"`
	IsActive       bool         `json:"is_active"`
	Priority       int          `json:"priority"`
	Conditions     []*Condition `json:"conditions"`
	Steps          []*Step      `json:"steps"`
	FormFields     []*FormField `json:"form_fields"`
	StepCount      int          `json:"step_count"`      // populated by ListTemplates (avoids N+1)
	ConditionCount int          `json:"condition_count"` // populated by ListTemplates (avoids N+1)
	CreatedBy      string       `json:"created_by"`
	CreatedByName  string       `json:"created_by_name,omitempty"` // display name; filled in by the REST layer
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// Condition defines a field-based rule for template matching.
type Condition struct {
	ID       string `json:"id"`
	Field    string `json:"field"`    // "amount", "service_type", "category"
	Operator string `json:"operator"` // "gt", "lt", "eq", "in", "between"
	Value    string `json:"value"`    // JSON-encoded value
}

// Step defines a single approval step within a template.
type Step struct {
	ID            string `json:"id"`
	StepOrder     int    `json:"step_order"`
	Name          string `json:"name"`
	ApproverType  string `json:"approver_type"`  // "specific_user", "role_in_dept", "department", "creator_manager"
	ApproverValue string `json:"approver_value"` // UA name, user_node_id, or "{creator_dept}" placeholder
	// ApproverName is who ApproverValue stands for, as a person would say it.
	// It is filled in on the way out by the REST layer and never stored.
	ApproverName  string `json:"approver_name,omitempty"`
	RequiredCount int    `json:"required_count"`
	TimeoutHours  int    `json:"timeout_hours"`
}

// Request represents a running approval request instance.
type Request struct {
	ID               string     `json:"id"`
	EntityType       string     `json:"entity_type"`
	EntityID         string     `json:"entity_id"`
	TemplateID       string     `json:"template_id"`
	TemplateName     string     `json:"template_name"`
	TemplateSnapshot string     `json:"template_snapshot,omitempty"` // JSON-encoded frozen template
	FormDataJSON     string     `json:"form_data_json"`              // JSON-encoded submitted form values
	CurrentStep      int        `json:"current_step"`
	Status           string     `json:"status"` // "pending", "approved", "rejected", "cancelled"
	ScopeOAID        string     `json:"scope_oa_id"`
	DepartmentID     string     `json:"department_id"`
	CreatedBy        string     `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at"`
	// Display names for the ids above, filled in on the way out by the REST
	// layer so the client never has to show an id. Never stored.
	CreatedByName  string `json:"created_by_name,omitempty"`
	DepartmentName string `json:"department_name,omitempty"`
}

// AssignmentRecord represents a denormalized user→request approval assignment.
type AssignmentRecord struct {
	ID          string     `json:"id"`
	RequestID   string     `json:"request_id"`
	StepOrder   int        `json:"step_order"`
	UserNodeID  string     `json:"user_node_id"`
	GrantSource string     `json:"grant_source"` // "direct", "role:KeToan_Chief", "department:KeToan_Dept"
	Status      string     `json:"status"`       // "pending", "approved", "rejected", "skipped", "revoked"
	ActedAt     *time.Time `json:"acted_at"`
	Comment     string     `json:"comment"`
	// UserName is who (or which role or department) UserNodeID is; filled in
	// by the REST layer, never stored.
	UserName string `json:"user_name,omitempty"`
}

// RequestWithAssignment pairs a request with the user's specific assignment.
// Used for the "pending" and "history" query tabs.
type RequestWithAssignment struct {
	Request    *Request          `json:"request"`
	Assignment *AssignmentRecord `json:"assignment"`
}

// AuditEntry represents a single append-only audit log record.
type AuditEntry struct {
	ID          string    `json:"id"`
	RequestID   string    `json:"request_id"`
	Action      string    `json:"action"` // "created", "assigned", "approved", "rejected", etc.
	ActorNodeID string    `json:"actor_node_id"`
	StepOrder   int       `json:"step_order"`
	DetailJSON  string    `json:"detail_json"`
	IPAddress   string    `json:"ip_address"`
	CreatedAt   time.Time `json:"created_at"`
	// ActorName is the display name of ActorNodeID; filled in by the REST
	// layer, never stored. Empty for system entries (step advanced, completed).
	ActorName string `json:"actor_name,omitempty"`
}

// RequestDetail is one request opened for reading: the request itself, the
// chain of steps it runs through and every approver's assignment, not only the
// caller's. Steps and form fields come from the template as it was when the
// request was made, so a later edit of the template does not rewrite history.
type RequestDetail struct {
	Request     *Request            `json:"request"`
	Steps       []*Step             `json:"steps"`
	FormFields  []*FormField        `json:"form_fields"`
	Assignments []*AssignmentRecord `json:"assignments"`
	// CanAct: it is the caller's turn, directly or through a role or department
	// they belong to right now, and they have not yet acted on this step.
	CanAct bool `json:"can_act"`
}
