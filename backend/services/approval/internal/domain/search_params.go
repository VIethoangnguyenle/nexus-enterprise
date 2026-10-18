package domain

// SearchParams carries every filter the search query accepts, plus the access
// control context the SQL needs to restrict results to what the caller may see.
type SearchParams struct {
	// Text search (required, min 2 chars).
	Query string

	// Access control context — populated by the domain from the policy client.
	UserNodeID       string
	GroupNodeIDs     []string
	AccessibleScopes []string

	// Optional filters from query params.
	Status     string // pending|approved|rejected|cancelled
	TemplateID string
	CreatedBy  string // NGAC node ID of the requester
	From       string // YYYY-MM-DD inclusive
	To         string // YYYY-MM-DD inclusive
	Limit      int    // default 20, max 50
}
