package texts_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/realtime"
	"ngac-platform/services/document/internal/texts"
)

// visibleEmitter records events and, at the moment of each Emit, whether the
// document the event names can be read from the database.
type visibleEmitter struct {
	realtime.Recorder
	f       *fixture
	version []int // version of the row seen at emit time; 0 when absent
}

func (v *visibleEmitter) Emit(e realtime.Event) {
	n := 0
	if len(e.IDs) == 1 {
		if d, err := texts.NewStore(v.f.pool).Get(context.Background(), e.IDs[0]); err == nil {
			n = d.Version
		}
	}
	v.version = append(v.version, n)
	v.Recorder.Emit(e)
}

func emitting(f *fixture) (*visibleEmitter, texts.Caller) {
	em := &visibleEmitter{f: f}
	f.svc.SetEmitter(em)
	who := f.owner
	who.TenantID = f.wsID
	return em, who
}

func TestEmit_CreateAnnouncesTheNewDocumentAfterItIsStored(t *testing.T) {
	f := newFixture(t)
	f.policy.grant(f.owner.NGACNodeID, f.oaA, ngac.OpWrite)
	em, who := emitting(f)

	d, err := f.svc.Create(bg, who, f.wsID, f.folderA, "Quy trình")

	require.NoError(t, err)
	evs := em.Events()
	require.Len(t, evs, 1)
	assert.Equal(t, realtime.DomainDocument, evs[0].Domain)
	assert.Equal(t, realtime.KindCreated, evs[0].Kind)
	assert.Equal(t, []string{d.ID}, evs[0].IDs)
	assert.Equal(t, f.wsID, evs[0].WorkspaceID)
	assert.Equal(t, f.wsID, evs[0].TenantID)
	assert.Equal(t, f.folderA, evs[0].ParentID)
	assert.Equal(t, f.owner.UserID, evs[0].ActorUserID)
	assert.Equal(t, []int{1}, em.version, "the row was readable when the event went out")
}

func TestEmit_UpdateAnnouncesTheNewVersion(t *testing.T) {
	f := newFixture(t)
	f.policy.grant(f.owner.NGACNodeID, f.oaA, ngac.OpWrite, ngac.OpRead)
	_, who := emitting(f)
	d, err := f.svc.Create(bg, who, f.wsID, f.folderA, "A")
	require.NoError(t, err)
	em, who := emitting(f) // fresh recorder after the create

	title := "B"
	_, err = f.svc.Update(bg, who, d.ID, texts.Change{BaseVersion: 1, Title: &title})

	require.NoError(t, err)
	evs := em.Events()
	require.Len(t, evs, 1)
	assert.Equal(t, realtime.KindUpdated, evs[0].Kind)
	assert.Equal(t, []string{d.ID}, evs[0].IDs)
	assert.Equal(t, f.folderA, evs[0].ParentID)
	assert.Equal(t, []int{2}, em.version, "the bumped version was committed before the event")
}

func TestEmit_DeleteAnnouncesAfterTheRowIsGone(t *testing.T) {
	f := newFixture(t)
	f.policy.grant(f.owner.NGACNodeID, f.oaA, ngac.OpWrite)
	_, who := emitting(f)
	d, err := f.svc.Create(bg, who, f.wsID, f.folderA, "A")
	require.NoError(t, err)
	em, who := emitting(f)

	require.NoError(t, f.svc.Delete(bg, who, d.ID))

	evs := em.Events()
	require.Len(t, evs, 1)
	assert.Equal(t, realtime.KindDeleted, evs[0].Kind)
	assert.Equal(t, []string{d.ID}, evs[0].IDs)
	assert.Equal(t, f.folderA, evs[0].ParentID)
	assert.Equal(t, []int{0}, em.version, "the row was already gone")
}

func TestEmit_NothingOnFailedWrites(t *testing.T) {
	f := newFixture(t)
	f.policy.grant(f.owner.NGACNodeID, f.oaA, ngac.OpWrite, ngac.OpRead)
	_, who := emitting(f)
	d, err := f.svc.Create(bg, who, f.wsID, f.folderA, "A")
	require.NoError(t, err)
	em, who := emitting(f)
	stranger := f.other
	stranger.TenantID = f.wsID
	bad := "x\x00y"
	ok := "fine"
	status := "bogus"

	// Forbidden.
	_, err = f.svc.Create(bg, stranger, f.wsID, f.folderA, "x")
	assertDenied(t, err)
	_, err = f.svc.Update(bg, stranger, d.ID, texts.Change{BaseVersion: 1, Title: &ok})
	assertDenied(t, err)
	assertDenied(t, f.svc.Delete(bg, stranger, d.ID))
	// Not found.
	_, err = f.svc.Update(bg, who, "no-such-doc", texts.Change{BaseVersion: 1, Title: &ok})
	assertDenied(t, err)
	assertDenied(t, f.svc.Delete(bg, who, "no-such-doc"))
	_, err = f.svc.Create(bg, who, f.wsID, "no-such-folder", "x")
	assertDenied(t, err)
	// Validation.
	_, err = f.svc.Update(bg, who, d.ID, texts.Change{BaseVersion: 1, Title: &bad})
	require.Error(t, err)
	_, err = f.svc.Update(bg, who, d.ID, texts.Change{BaseVersion: 1, Status: &status})
	require.Error(t, err)
	_, err = f.svc.Update(bg, who, d.ID, texts.Change{BaseVersion: 0, Title: &ok})
	require.Error(t, err)
	_, err = f.svc.Create(bg, who, f.wsID, f.folderA, "x\x00")
	require.Error(t, err)
	// Version conflict.
	_, err = f.svc.Update(bg, who, d.ID, texts.Change{BaseVersion: 7, Title: &ok})
	var conflict *texts.ConflictError
	require.ErrorAs(t, err, &conflict)
	// Policy unavailable.
	f.policy.fail = assert.AnError
	_, err = f.svc.Update(bg, who, d.ID, texts.Change{BaseVersion: 1, Title: &ok})
	require.Error(t, err)
	f.policy.fail = nil

	assert.Empty(t, em.Events(), "no failed path may announce anything")
}

func TestEmit_NilEmitterIsHarmless(t *testing.T) {
	f := newFixture(t)
	f.policy.grant(f.owner.NGACNodeID, f.oaA, ngac.OpWrite)
	f.svc.SetEmitter(nil)
	_, err := f.svc.Create(bg, f.owner, f.wsID, f.folderA, "A")
	require.NoError(t, err)
}
