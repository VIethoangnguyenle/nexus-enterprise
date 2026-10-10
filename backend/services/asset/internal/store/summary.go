package store

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// TypeCount is how many assets one type has.
type TypeCount struct {
	TypeID, TypeName string
	Count            int32
}

// Summary is the dashboard's counts over a set of asset types.
type Summary struct {
	Total   int32
	ByState map[string]int32
	ByType  []TypeCount
	// Holders is the number of distinct people holding an asset.
	Holders int32
	// MaintenanceOverdue counts assets that went into maintenance more than 14 days ago.
	MaintenanceOverdue int32
}

// Summary counts the workspace's assets of the given types. An empty set of
// types counts nothing: the gRPC layer passes the types the caller may read.
func (s *Store) Summary(ctx context.Context, workspaceID string, typeIDs []string) (*Summary, error) {
	sum := &Summary{ByState: map[string]int32{}}
	if len(typeIDs) == 0 {
		return sum, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT a.state, a.type_id, t.name, COUNT(*)
		   FROM assets a JOIN asset_types t ON t.id = a.type_id
		  WHERE a.workspace_id = $1 AND a.deleted = FALSE AND a.type_id = ANY($2)
		  GROUP BY a.state, a.type_id, t.name`, workspaceID, typeIDs)
	if err != nil {
		return nil, fmt.Errorf("summarising assets: %w", err)
	}
	defer rows.Close()
	byType := map[string]*TypeCount{}
	for rows.Next() {
		var state, typeID, typeName string
		var n int32
		if err := rows.Scan(&state, &typeID, &typeName, &n); err != nil {
			return nil, fmt.Errorf("scanning summary: %w", err)
		}
		sum.Total += n
		sum.ByState[state] += n
		tc := byType[typeID]
		if tc == nil {
			tc = &TypeCount{TypeID: typeID, TypeName: typeName}
			byType[typeID] = tc
		}
		tc.Count += n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, tc := range byType {
		sum.ByType = append(sum.ByType, *tc)
	}
	sort.Slice(sum.ByType, func(i, j int) bool {
		if sum.ByType[i].Count != sum.ByType[j].Count {
			return sum.ByType[i].Count > sum.ByType[j].Count
		}
		return sum.ByType[i].TypeName < sum.ByType[j].TypeName
	})
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT assigned_to) FROM assets
		  WHERE workspace_id = $1 AND deleted = FALSE AND type_id = ANY($2) AND state = 'assigned' AND assigned_to IS NOT NULL`,
		workspaceID, typeIDs).Scan(&sum.Holders); err != nil {
		return nil, fmt.Errorf("counting holders: %w", err)
	}
	// Since when an asset has been in maintenance: its latest step into it, else its last change.
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM assets a
		  WHERE a.workspace_id = $1 AND a.deleted = FALSE AND a.type_id = ANY($2) AND a.state = 'maintenance'
		    AND COALESCE((SELECT MAX(t.created_at) FROM asset_transitions t WHERE t.asset_id = a.id AND t.to_state = 'maintenance'), a.updated_at)
		        < NOW() - INTERVAL '14 days'`,
		workspaceID, typeIDs).Scan(&sum.MaintenanceOverdue); err != nil {
		return nil, fmt.Errorf("counting overdue maintenance: %w", err)
	}
	return sum, nil
}

// ActivityEntry is one lifecycle step across a workspace's assets.
type ActivityEntry struct {
	ID, AssetID, AssetName, TypeName string
	FromState, ToState, Action       string
	ActorID, ActorName               string
	SubjectUserID, SubjectName       string
	Comment                          string
	CreatedAt                        time.Time
	// RequestStatus is set for a decision on a request ("approved" or "rejected"), empty for a step.
	RequestStatus string
}

// ListActivity returns the newest steps on the workspace's assets of the given
// types, with the people involved named when they belong to the workspace.
func (s *Store) ListActivity(ctx context.Context, workspaceID string, typeIDs []string, limit int32) ([]*ActivityEntry, error) {
	if len(typeIDs) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx,
		`SELECT tr.id, tr.asset_id, a.name, t.name, tr.from_state, tr.to_state, tr.action,
		        tr.actor_id, `+personName("au", "atu")+`, COALESCE(tr.subject_user_id, ''), `+personName("su", "stu")+`,
		        tr.comment, tr.created_at
		   FROM asset_transitions tr
		   JOIN assets a ON a.id = tr.asset_id
		   JOIN asset_types t ON t.id = a.type_id`+
			memberJoin("tr.actor_id", "a.workspace_id", "au", "atu")+
			memberJoin("tr.subject_user_id", "a.workspace_id", "su", "stu")+`
		  WHERE a.workspace_id = $1 AND a.deleted = FALSE AND a.type_id = ANY($2)
		  ORDER BY tr.created_at DESC, tr.id LIMIT $3`, workspaceID, typeIDs, limit)
	if err != nil {
		return nil, fmt.Errorf("listing activity: %w", err)
	}
	defer rows.Close()
	var out []*ActivityEntry
	for rows.Next() {
		e := &ActivityEntry{}
		if err := rows.Scan(&e.ID, &e.AssetID, &e.AssetName, &e.TypeName, &e.FromState, &e.ToState, &e.Action,
			&e.ActorID, &e.ActorName, &e.SubjectUserID, &e.SubjectName, &e.Comment, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning activity: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListRequestDecisions returns the newest decisions (approved without an asset,
// or rejected) on requests for the given types. A request that was fulfilled
// shows as the hand-over step it produced, so it is not repeated here.
func (s *Store) ListRequestDecisions(ctx context.Context, workspaceID string, typeIDs []string, limit int32) ([]*ActivityEntry, error) {
	if len(typeIDs) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx,
		`SELECT r.id, t.name, r.status, COALESCE(r.approver_id, ''), `+personName("au", "atu")+`,
		        r.requester_id, `+personName("ru", "rtu")+`, r.approver_comment, r.updated_at
		   FROM asset_requests r
		   JOIN asset_types t ON t.id = r.type_id`+
			memberJoin("r.approver_id", "r.workspace_id", "au", "atu")+
			memberJoin("r.requester_id", "r.workspace_id", "ru", "rtu")+`
		  WHERE r.workspace_id = $1 AND r.type_id = ANY($2) AND r.status IN ('approved', 'rejected')
		  ORDER BY r.updated_at DESC, r.id LIMIT $3`, workspaceID, typeIDs, limit)
	if err != nil {
		return nil, fmt.Errorf("listing request decisions: %w", err)
	}
	defer rows.Close()
	var out []*ActivityEntry
	for rows.Next() {
		e := &ActivityEntry{}
		if err := rows.Scan(&e.ID, &e.TypeName, &e.RequestStatus, &e.ActorID, &e.ActorName, &e.SubjectUserID, &e.SubjectName, &e.Comment, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning request decision: %w", err)
		}
		e.AssetName = e.TypeName
		e.Action = "request_" + e.RequestStatus
		out = append(out, e)
	}
	return out, rows.Err()
}
