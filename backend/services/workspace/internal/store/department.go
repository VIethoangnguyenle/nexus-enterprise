package store

import "time"

// Department is the internal DB representation of a department row.
type Department struct {
	ID          string
	WorkspaceID string
	Name        string
	ParentID    *string
	NGACUaID    string
	SortOrder   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
