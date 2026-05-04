package domain

import "time"

// FormField defines a single dynamic form field within a template.
// Templates use these to specify what data submitters must provide.
type FormField struct {
	Label       string `json:"label"`
	FieldType   string `json:"field_type"` // "text", "number", "currency", "date", "select", "textarea"
	Required    bool   `json:"required"`
	Options     string `json:"options,omitempty"`     // comma-separated options for "select" type
	FieldOrder  int    `json:"field_order"`           // display order
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
}
