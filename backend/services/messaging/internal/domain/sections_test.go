package domain_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

// sectionWorld is a workspace the caller belongs to, with two readable channels.
type sectionWorld struct {
	*talk
	me       domain.Caller
	chA, chB string
}

func newSectionWorld(t *testing.T) *sectionWorld {
	t.Helper()
	k := newTalk(t, "sections")
	ctx := context.Background()
	// The workspace-level attribute "being in the workspace" is read on.
	docs := fmt.Sprintf("test-docs-oa-%d", time.Now().UnixNano())
	_, err := k.f.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2)`, docs, ngac.TypeOA)
	require.NoError(t, err)
	_, err = k.f.pool.Exec(ctx, `UPDATE workspaces SET documents_oa_id = $2 WHERE id = $1`, k.f.wsID, docs)
	require.NoError(t, err)
	t.Cleanup(func() {
		k.f.pool.Exec(ctx, `DELETE FROM chat_sections WHERE workspace_id = $1`, k.f.wsID)
		k.f.pool.Exec(ctx, `UPDATE workspaces SET documents_oa_id = NULL WHERE id = $1`, k.f.wsID)
		k.f.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, docs)
	})
	k.f.read.allow[grant{userNode, docs, ngac.OpRead}] = true
	chB, oaB := k.f.insertChannel(t, "sections-b")
	k.f.read.allow[grant{userNode, oaB, ngac.OpRead}] = true
	return &sectionWorld{
		talk: k, chA: k.chID, chB: chB,
		me: domain.Caller{UserID: k.f.userID, NodeID: userNode, TenantID: k.f.wsID},
	}
}

func names(secs []*domain.Section) []string {
	out := make([]string, 0, len(secs))
	for _, s := range secs {
		out = append(out, s.Name)
	}
	return out
}

func TestSections_CreateRenameReorderAndPlace(t *testing.T) {
	w := newSectionWorld(t)
	ctx := context.Background()
	svc := w.f.svc

	work, err := svc.CreateSection(ctx, w.f.wsID, w.me, domain.NewSection{Name: "Công việc"})
	require.NoError(t, err)
	home, err := svc.CreateSection(ctx, w.f.wsID, w.me, domain.NewSection{Name: "Nhà"})
	require.NoError(t, err)

	_, err = svc.CreateSection(ctx, w.f.wsID, w.me, domain.NewSection{Name: "công VIỆC"})
	require.ErrorIs(t, err, domain.ErrAlreadyExists, "names are unique whatever their case")
	_, err = svc.CreateSection(ctx, w.f.wsID, w.me, domain.NewSection{Name: "  "})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	_, err = svc.CreateSection(ctx, w.f.wsID, w.me, domain.NewSection{Name: strings.Repeat("a", 41)})
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	require.NoError(t, svc.RenameSection(ctx, w.f.wsID, w.me, home.ID, "Gia đình"))
	require.NoError(t, svc.MoveSection(ctx, w.f.wsID, w.me, work.ID, "down"))
	secs, err := svc.ListSections(ctx, w.f.wsID, w.me)
	require.NoError(t, err)
	assert.Equal(t, []string{"Gia đình", "Công việc"}, names(secs))

	// A conversation is in at most one section: placing it again moves it.
	require.NoError(t, svc.PlaceChannel(ctx, w.f.wsID, w.me, work.ID, w.chA))
	require.NoError(t, svc.PlaceChannel(ctx, w.f.wsID, w.me, home.ID, w.chA))
	require.NoError(t, svc.PlaceChannel(ctx, w.f.wsID, w.me, work.ID, w.chB))
	secs, _ = svc.ListSections(ctx, w.f.wsID, w.me)
	assert.Equal(t, []string{w.chA}, secs[0].ChannelIDs)
	assert.Equal(t, []string{w.chB}, secs[1].ChannelIDs)

	require.NoError(t, svc.UnplaceChannel(ctx, w.f.wsID, w.me, w.chB))
	secs, _ = svc.ListSections(ctx, w.f.wsID, w.me)
	assert.Empty(t, secs[1].ChannelIDs)
}

// Deleting a section returns what Undo needs; creating it again from that
// snapshot restores its name, place and conversations.
func TestSections_DeleteAndUndo(t *testing.T) {
	w := newSectionWorld(t)
	ctx := context.Background()
	svc := w.f.svc
	first, err := svc.CreateSection(ctx, w.f.wsID, w.me, domain.NewSection{Name: "Một"})
	require.NoError(t, err)
	_, err = svc.CreateSection(ctx, w.f.wsID, w.me, domain.NewSection{Name: "Hai"})
	require.NoError(t, err)
	require.NoError(t, svc.PlaceChannel(ctx, w.f.wsID, w.me, first.ID, w.chA))

	gone, err := svc.DeleteSection(ctx, w.f.wsID, w.me, first.ID)
	require.NoError(t, err)
	secs, _ := svc.ListSections(ctx, w.f.wsID, w.me)
	assert.Equal(t, []string{"Hai"}, names(secs), "its conversations return to the default lists")

	pos := gone.Position
	_, err = svc.CreateSection(ctx, w.f.wsID, w.me, domain.NewSection{Name: gone.Name, Position: &pos, ChannelIDs: gone.ChannelIDs})
	require.NoError(t, err)
	secs, _ = svc.ListSections(ctx, w.f.wsID, w.me)
	assert.Equal(t, []string{"Một", "Hai"}, names(secs))
	assert.Equal(t, []string{w.chA}, secs[0].ChannelIDs)
}

func TestSections_Deny(t *testing.T) {
	w := newSectionWorld(t)
	ctx := context.Background()
	svc := w.f.svc
	mine, err := svc.CreateSection(ctx, w.f.wsID, w.me, domain.NewSection{Name: "Của tôi"})
	require.NoError(t, err)

	// Someone else's section does not exist for this caller.
	otherID, _ := testutil.CreateUser(t, w.f.pool)
	otherNode := "ngac-other-sections"
	w.f.read.allow[grant{otherNode, w.oaID, ngac.OpRead}] = true
	other := domain.Caller{UserID: otherID, NodeID: otherNode, TenantID: w.f.wsID}
	var docs string
	require.NoError(t, w.f.pool.QueryRow(ctx, `SELECT documents_oa_id FROM workspaces WHERE id = $1`, w.f.wsID).Scan(&docs))
	w.f.read.allow[grant{otherNode, docs, ngac.OpRead}] = true
	require.ErrorIs(t, svc.RenameSection(ctx, w.f.wsID, other, mine.ID, "chiếm"), domain.ErrNotFound)
	require.ErrorIs(t, svc.PlaceChannel(ctx, w.f.wsID, other, mine.ID, w.chA), domain.ErrNotFound)
	_, err = svc.DeleteSection(ctx, w.f.wsID, other, mine.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	theirs, err := svc.ListSections(ctx, w.f.wsID, other)
	require.NoError(t, err)
	assert.Empty(t, theirs, "sections are personal")

	// A channel the caller cannot read cannot be placed.
	closed, _ := w.f.insertChannel(t, "sections-closed")
	require.ErrorIs(t, svc.PlaceChannel(ctx, w.f.wsID, w.me, mine.ID, closed), domain.ErrAccessDenied)

	// A channel of another workspace cannot be placed here, even a readable one.
	otherWS, _ := testutil.CreateWorkspace(t, w.f.pool, w.f.userID)
	foreign := fmt.Sprintf("test-foreign-ch-%d", time.Now().UnixNano())
	_, err = w.f.pool.Exec(ctx, `INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, created_at)
		VALUES ($1, 'foreign', 'workspace', $2, $3, now())`, foreign, otherWS, w.oaID)
	require.NoError(t, err)
	t.Cleanup(func() { w.f.pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, foreign) })
	require.ErrorIs(t, svc.PlaceChannel(ctx, w.f.wsID, w.me, mine.ID, foreign), domain.ErrInvalidInput)

	// Outside the workspace (another tenant, or no read on it): nothing at all.
	stranger := domain.Caller{UserID: w.f.userID, NodeID: "ngac-stranger", TenantID: w.f.wsID}
	_, err = svc.ListSections(ctx, w.f.wsID, stranger)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	wrongTenant := domain.Caller{UserID: w.f.userID, NodeID: userNode, TenantID: otherWS}
	_, err = svc.CreateSection(ctx, w.f.wsID, wrongTenant, domain.NewSection{Name: "x"})
	require.ErrorIs(t, err, domain.ErrAccessDenied)
}
