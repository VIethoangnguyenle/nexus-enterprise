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

// Deny reasons — typed constants for PDP communication.
// Used by access.go (producer) and decision_engine.go (consumer).
const (
	DenyReasonNodeNotFound  = "node_not_found"
	DenyReasonNoAssociation = "no_association_path"
)

// Decision outcomes — used across PDP, cache, and gRPC layers.
const (
	DecisionAllow = "ALLOW"
	DecisionDeny  = "DENY"
)

// Version scope constants — used by cache versioning and invalidation.
const ScopeGlobal = "global"

// WorkspaceScope returns the version scope key for a tenant workspace.
func WorkspaceScope(wsID string) string {
	return "ws:" + wsID
}

// Operations are dynamic — registered at runtime via RegisterOperations RPC.
// Consumers define their own operations (e.g. "read", "approve", "transfer").
// No hardcoded constants in the generic policy service.

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
	Path               []string           `json:"path,omitempty"`
	PolicyClasses      []string           `json:"policy_classes,omitempty"`
	UserAttributes     []string           `json:"user_attributes,omitempty"`
	ObjectAttributes   []string           `json:"object_attributes,omitempty"`
	Reason             string             `json:"reason,omitempty"`
	ProhibitionDenied  *ProhibitionDenial `json:"prohibition_denied,omitempty"`
}

// ProhibitionDenial details when a prohibition denied access
type ProhibitionDenial struct {
	ProhibitionName string `json:"prohibition_name"`
	SubjectID       string `json:"subject_id"`
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
