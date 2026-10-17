package domain

import (
	"context"
	"sort"
	"sync"
	"time"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/realtime"
	"ngac-platform/services/calendar/internal/store"
)

// memStore is the store in memory.
type memStore struct {
	mu        sync.Mutex
	workspace *store.Workspace
	people    map[string]*store.Person // by node
	channels  map[string]*store.Channel
	members   map[string][]string // channel id -> member nodes
	calendars map[string]*store.Calendar
	subs      map[[2]string]*store.Subscription
	shares    map[[2]string]*store.Share
	// onInsert runs inside InsertCalendar before the row is stored, to stage a race.
	onInsert func(*store.Calendar)
}

func newMemStore(ws *store.Workspace) *memStore {
	return &memStore{
		workspace: ws, people: map[string]*store.Person{}, channels: map[string]*store.Channel{},
		members: map[string][]string{}, calendars: map[string]*store.Calendar{},
		subs: map[[2]string]*store.Subscription{}, shares: map[[2]string]*store.Share{},
	}
}

func (m *memStore) Workspace(_ context.Context, id string) (*store.Workspace, error) {
	if m.workspace == nil || m.workspace.ID != id {
		return nil, store.ErrNotFound
	}
	return m.workspace, nil
}

func (m *memStore) PersonByNode(_ context.Context, _ string, node string) (*store.Person, error) {
	if p, ok := m.people[node]; ok {
		return p, nil
	}
	return nil, store.ErrNotFound
}

func (m *memStore) People(_ context.Context, _ string, nodes []string) (map[string]*store.Person, error) {
	out := map[string]*store.Person{}
	for _, n := range nodes {
		if p, ok := m.people[n]; ok {
			out[n] = p
		}
	}
	return out, nil
}

func (m *memStore) Channel(_ context.Context, id string) (*store.Channel, error) {
	if c, ok := m.channels[id]; ok {
		return c, nil
	}
	return nil, store.ErrNotFound
}

// name resolves a calendar's name the way the SQL does.
func (m *memStore) resolve(c store.Calendar) *store.Calendar {
	switch c.Kind {
	case store.KindPersonal:
		if p, ok := m.people[c.OwnerNodeID]; ok {
			c.Name, c.Owner = p.DisplayName, p
		}
	case store.KindWorkspace:
		c.Name = m.workspace.Name
	case store.KindChannel:
		c.Name = m.channels[c.ChannelID].Name
	}
	return &c
}

func (m *memStore) CalendarByID(_ context.Context, id string) (*store.Calendar, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.calendars[id]; ok {
		return m.resolve(*c), nil
	}
	return nil, store.ErrNotFound
}

func (m *memStore) CalendarFor(_ context.Context, ws, kind, owner, channel string) (*store.Calendar, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.calendars {
		if c.WorkspaceID != ws || c.Kind != kind {
			continue
		}
		if (kind == store.KindPersonal && c.OwnerNodeID == owner) || (kind == store.KindChannel && c.ChannelID == channel) || kind == store.KindWorkspace {
			return m.resolve(*c), nil
		}
	}
	return nil, store.ErrNotFound
}

func (m *memStore) InsertCalendar(_ context.Context, c *store.Calendar) error {
	if m.onInsert != nil {
		fn := m.onInsert
		m.onInsert = nil
		fn(c)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.calendars {
		if e.ID == c.ID || (e.Kind == c.Kind && e.WorkspaceID == c.WorkspaceID && e.OwnerNodeID == c.OwnerNodeID && e.ChannelID == c.ChannelID) {
			return store.ErrAlreadyExists
		}
	}
	cp := *c
	m.calendars[c.ID] = &cp
	return nil
}

func (m *memStore) MemberChannelCalendars(_ context.Context, ws, node string) ([]*store.Calendar, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*store.Calendar
	for _, c := range m.calendars {
		if c.Kind != store.KindChannel || c.WorkspaceID != ws || m.channels[c.ChannelID].Type == "dm" {
			continue
		}
		for _, mem := range m.members[c.ChannelID] {
			if mem == node {
				out = append(out, m.resolve(*c))
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *memStore) Subscriptions(_ context.Context, ws, node string) ([]*store.Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*store.Subscription
	for k, s := range m.subs {
		if k[0] == node && m.calendars[k[1]].WorkspaceID == ws {
			cp := *s
			cp.Calendar = m.resolve(*m.calendars[k[1]])
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Calendar.ID < out[j].Calendar.ID })
	return out, nil
}

func (m *memStore) SubscriptionOf(_ context.Context, node, cal string) (*store.Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.subs[[2]string{node, cal}]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, store.ErrNotFound
}

func (m *memStore) SaveSubscription(_ context.Context, node, cal string, color *int16, visible *bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.subs[[2]string{node, cal}]
	if !ok {
		s = &store.Subscription{Visible: true}
		m.subs[[2]string{node, cal}] = s
	}
	if color != nil {
		s.Color = color
	}
	if visible != nil {
		s.Visible = *visible
	}
	return nil
}

func (m *memStore) DeleteSubscription(_ context.Context, node, cal string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.subs[[2]string{node, cal}]
	delete(m.subs, [2]string{node, cal})
	return ok, nil
}

func (m *memStore) Shares(_ context.Context, cal string) ([]*store.Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*store.Share
	for k, s := range m.shares {
		if k[0] == cal {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GranteeNodeID < out[j].GranteeNodeID })
	return out, nil
}

func (m *memStore) ShareOf(_ context.Context, cal, grantee string) (*store.Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.shares[[2]string{cal, grantee}]; ok {
		return s, nil
	}
	return nil, store.ErrNotFound
}

func (m *memStore) SaveShare(_ context.Context, sh *store.Share) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *sh
	cp.CreatedAt = time.Unix(1, 0)
	m.shares[[2]string{sh.CalendarID, sh.GranteeNodeID}] = &cp
	return nil
}

func (m *memStore) DeleteShare(_ context.Context, cal, grantee string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.shares[[2]string{cal, grantee}]
	delete(m.shares, [2]string{cal, grantee})
	return ok, nil
}

// callerCtx is a request from the person with this user node, scoped to ws1.
func callerCtx(node string) context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "user-" + node, NGACNodeID: node, TenantID: "ws1"})
}

var _ Store = (*memStore)(nil)
var _ realtime.Emitter = (*realtime.Recorder)(nil)
