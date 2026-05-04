// Package store provides database access for the workspace service.
package store

import "time"

// Workspace is the internal DB representation of a workspace row.
type Workspace struct {
	ID        string
	Name      string
	Desc      string
	OwnerID   string
	NGACPcID  string
	CreatedAt time.Time
}
