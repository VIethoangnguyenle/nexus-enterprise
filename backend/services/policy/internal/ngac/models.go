package ngac

import "time"

// Node types
const (
	NodeTypeUser          = "U"
	NodeTypeUserAttribute = "UA"
	NodeTypeObject        = "O"
	NodeTypeObjectAttr    = "OA"
	NodeTypePolicyClass   = "PC"
)

// Operations
const (
	OpRead          = "read"
	OpWrite         = "write"
	OpApprove       = "approve"
	OpUpload        = "upload"
	OpShare         = "share"
	OpManage        = "manage"
	OpInvite        = "invite"
	OpCreateChannel = "create_channel"
)

// NGACNode represents any node in the NGAC graph
type NGACNode struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	NodeType   string            `json:"node_type"`
	Properties map[string]string `json:"properties,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
}

// Assignment represents a containment edge (child → parent)
type Assignment struct {
	ID       string `json:"id"`
	ChildID  string `json:"child_id"`
	ParentID string `json:"parent_id"`
}

// Association represents a permission edge (UA → OA with operations)
type Association struct {
	ID         string   `json:"id"`
	UAID       string   `json:"ua_id"`
	OAID       string   `json:"oa_id"`
	Operations []string `json:"operations"`
}

// AccessDecision is the result of an access check
type AccessDecision struct {
	Decision    string            `json:"decision"` // "ALLOW" or "DENY"
	User        string            `json:"user"`
	Object      string            `json:"object"`
	Operation   string            `json:"operation"`
	Explanation AccessExplanation `json:"explanation"`
}

// AccessExplanation provides details about why access was granted or denied
type AccessExplanation struct {
	Path               []string          `json:"path,omitempty"`
	PolicyClass        string            `json:"policy_class,omitempty"`
	UserAttributes     []string          `json:"user_attributes,omitempty"`
	ObjectAttributes   []string          `json:"object_attributes,omitempty"`
	Reason             string            `json:"reason,omitempty"`
	ConstraintsChecked []string          `json:"constraints_checked,omitempty"`
	ConstraintDenied   *ConstraintDenial `json:"constraint_denied,omitempty"`
}

// ConstraintDenial details when a constraint blocked access
type ConstraintDenial struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// ValidAssignments defines which node type pairs are valid for assignments
var ValidAssignments = map[string][]string{
	NodeTypeUser:          {NodeTypeUserAttribute},
	NodeTypeUserAttribute: {NodeTypeUserAttribute, NodeTypePolicyClass},
	NodeTypeObject:        {NodeTypeObjectAttr},
	NodeTypeObjectAttr:    {NodeTypeObjectAttr, NodeTypePolicyClass},
}

func IsValidNodeType(t string) bool {
	switch t {
	case NodeTypeUser, NodeTypeUserAttribute, NodeTypeObject, NodeTypeObjectAttr, NodeTypePolicyClass:
		return true
	}
	return false
}

// nameTypeKey builds a composite key for the nameType index.
// Enables O(1) node lookup by name+type instead of O(N) linear scan.
func nameTypeKey(name, nodeType string) string {
	return name + "\x00" + nodeType
}

func IsValidAssignment(childType, parentType string) bool {
	allowed, ok := ValidAssignments[childType]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == parentType {
			return true
		}
	}
	return false
}
