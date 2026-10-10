// Package store provides database access for the workspace service.
package store

import "time"

// Workspace is the internal DB representation of a workspace row.
type Workspace struct {
	ID       string
	Name     string
	Desc     string
	OwnerID  string
	NGACPcID string
	// DocumentsOAID is the workspace's Documents OA. The drive roots itself there,
	// and must not have to rediscover it by looking at node names.
	DocumentsOAID string
	CreatedAt     time.Time
}
