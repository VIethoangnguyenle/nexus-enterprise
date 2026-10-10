package texts_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	"ngac-platform/services/document/internal/texts"
)

var bg = context.Background()

func assertDenied(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	assert.True(t, errors.Is(err, httputil.ErrAccessDenied), "err = %v", err)
}

// ---- create ----

func TestCreate_WriteOnTheFolderOAAllows(t *testing.T) {
	f := newFixture(t)
	f.policy.grant(f.owner.NGACNodeID, f.oaA, ngac.OpWrite)

	d, err := f.svc.Create(bg, f.owner, f.wsID, f.folderA, "Quy trình")

	require.NoError(t, err)
	assert.Equal(t, "Quy trình", d.Title)
	assert.Equal(t, texts.StatusDraft, d.Status)
	assert.Equal(t, 1, d.Version)
	assert.Equal(t, f.owner.UserID, d.OwnerID)
	assert.Equal(t, "Lê Thị Hoa", d.OwnerName)
	assert.Contains(t, f.policy.checks, [3]string{f.owner.NGACNodeID, f.oaA, ngac.OpWrite}, "the check lands on the folder's OA")
}

func TestCreate_DeniedOffTheFolderOA(t *testing.T) {
	cases := map[string]func(f *fixture){
		"read, not write, on the folder": func(f *fixture) { f.policy.grant(f.owner.NGACNodeID, f.oaA, ngac.OpRead) },
		"write on another folder":        func(f *fixture) { f.policy.grant(f.owner.NGACNodeID, f.oaB, ngac.OpWrite) },
		"write on the Documents OA only": func(f *fixture) { f.policy.grant(f.owner.NGACNodeID, f.docsOA, ngac.OpWrite) },
		"manage, not write":              func(f *fixture) { f.policy.grant(f.owner.NGACNodeID, f.oaA, ngac.OpManage) },
		"nothing":                        func(*fixture) {},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			setup(f)
			_, err := f.svc.Create(bg, f.owner, f.wsID, f.folderA, "x")
			assertDenied(t, err)
		})
	}
}

func TestCreate_DeniedWithoutCallerOrWhenPolicyFails(t *testing.T) {
	f := newFixture(t)
	f.policy.grant(f.owner.NGACNodeID, f.oaA, ngac.OpWrite)

	_, err := f.svc.Create(bg, texts.Caller{}, f.wsID, f.folderA, "x")
	assertDenied(t, err)
	_, err = f.svc.Create(bg, texts.Caller{UserID: f.owner.UserID}, f.wsID, f.folderA, "x")
	assertDenied(t, err)

	f.policy.fail = errors.New("policy down")
	_, err = f.svc.Create(bg, f.owner, f.wsID, f.folderA, "x")
	assertDenied(t, err)
}

func TestCreate_FolderOfAnotherWorkspaceOrGoneDenies(t *testing.T) {
	f := newFixture(t)
	f.policy.grant(f.owner.NGACNodeID, f.oaA, ngac.OpWrite)
	// The folder is real and the caller can write it, but it is not in this workspace.
	otherWS, _ := testutilWorkspace(t, f)
	_, err := f.svc.Create(bg, f.owner, otherWS, f.folderA, "x")
	assertDenied(t, err)

	f.exec(`UPDATE drive_items SET status = 'trashed' WHERE id = $1`, f.folderA)
	_, err = f.svc.Create(bg, f.owner, f.wsID, f.folderA, "x")
	assertDenied(t, err)

	_, err = f.svc.Create(bg, f.owner, f.wsID, "no-such-folder", "x")
	assertDenied(t, err)
}

func TestCreate_AtTheTopUsesTheDocumentsOA(t *testing.T) {
	f := newFixture(t)
	f.policy.grant(f.owner.NGACNodeID, f.docsOA, ngac.OpWrite)

	d, err := f.svc.Create(bg, f.owner, f.wsID, "", "")

	require.NoError(t, err)
	assert.Equal(t, texts.DefaultTitle, d.Title)
	assert.Equal(t, f.docsOA, d.OAID)

	g := newFixture(t)
	g.policy.grant(g.owner.NGACNodeID, g.oaA, ngac.OpWrite) // a folder grant is not a grant on the top
	_, err = g.svc.Create(bg, g.owner, g.wsID, "", "x")
	assertDenied(t, err)
}

func TestCreate_RejectsBadTitles(t *testing.T) {
	f := newFixture(t)
	f.policy.grant(f.owner.NGACNodeID, f.oaA, ngac.OpWrite)
	for name, title := range map[string]string{
		"too long": strings.Repeat("a", texts.MaxTitleRunes+1),
		"NUL":      "a\x00b",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := f.svc.Create(bg, f.owner, f.wsID, f.folderA, title)
			require.Error(t, err)
			assert.True(t, errors.Is(err, httputil.ErrInvalidInput), "err = %v", err)
		})
	}
}

// ---- get ----

func TestGet_ReadOnTheFolderOAAllowsAndReportsWrite(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "Quy trình")
	f.policy.grant(f.reader.NGACNodeID, f.oaA, ngac.OpRead)

	got, err := f.svc.Get(bg, f.reader, d.ID)
	require.NoError(t, err)
	assert.Equal(t, "Quy trình", got.Title)
	assert.False(t, got.CanWrite, "a reader is told it cannot write")

	own, err := f.svc.Get(bg, f.owner, d.ID)
	require.NoError(t, err)
	assert.True(t, own.CanWrite)
}

func TestGet_DeniedOffTheFolderOA(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "x")
	f.policy.grant(f.reader.NGACNodeID, f.oaB, ngac.OpRead)  // another folder
	f.policy.grant(f.reader.NGACNodeID, f.oaA, ngac.OpWrite) // write without read

	_, err := f.svc.Get(bg, f.reader, d.ID)
	assertDenied(t, err)
	_, err = f.svc.Get(bg, f.other, d.ID)
	assertDenied(t, err)
	_, err = f.svc.Get(bg, texts.Caller{}, d.ID)
	assertDenied(t, err)
}

func TestGet_MissingAndForbiddenLookAlike(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "x")
	_, errForbidden := f.svc.Get(bg, f.other, d.ID)
	_, errMissing := f.svc.Get(bg, f.other, "00000000-0000-0000-0000-000000000000")
	assertDenied(t, errForbidden)
	assertDenied(t, errMissing)
	assert.Equal(t, errForbidden.Error(), errMissing.Error(), "the answer does not say whether the id exists")
}

func TestGet_PolicyOutageDenies(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "x")
	f.policy.fail = errors.New("policy down")
	_, err := f.svc.Get(bg, f.owner, d.ID)
	assertDenied(t, err)
}

func TestGet_DocumentOfARemovedFolderIsUnreachable(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "x")
	f.exec(`UPDATE drive_items SET status = 'trashed' WHERE id = $1`, f.folderA)

	_, err := f.svc.Get(bg, f.owner, d.ID) // the owner still holds read and write on the OA
	assertDenied(t, err)
}

func TestGet_OwnerWhoLeftTheWorkspaceIsNotNamed(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "x")
	require.Equal(t, "Lê Thị Hoa", d.OwnerName)
	f.exec(`DELETE FROM tenant_users WHERE tenant_id = $1 AND user_id = $2`, f.wsID, f.owner.UserID)

	got, err := f.svc.Get(bg, f.owner, d.ID)
	require.NoError(t, err)
	assert.Empty(t, got.OwnerName, "a person outside the workspace is not named")
}

// ---- list ----

func TestList_ShowsOnlyWhatTheCallerMayRead(t *testing.T) {
	f := newFixture(t)
	a := f.doc(f.folderA, f.oaA, "In A")
	f.doc(f.folderB, f.oaB, "In B")
	f.policy.grant(f.reader.NGACNodeID, f.oaA, ngac.OpRead)

	got, err := f.list(f.reader, f.wsID, "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, a.ID, got[0].ID)
	assert.Empty(t, got[0].Content, "a listing does not carry content")
	assert.False(t, got[0].CanWrite, "read without write is reported as read-only")

	f.policy.grant(f.reader.NGACNodeID, f.oaA, ngac.OpWrite)
	got, err = f.list(f.reader, f.wsID, "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.True(t, got[0].CanWrite)

	none, err := f.list(f.other, f.wsID, "")
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestList_FailsClosed(t *testing.T) {
	f := newFixture(t)
	f.doc(f.folderA, f.oaA, "x")
	f.policy.fail = errors.New("policy down")

	got, err := f.list(f.owner, f.wsID, "")
	require.Error(t, err)
	assert.Empty(t, got, "an unreadable answer lists nothing")

	_, err = f.list(texts.Caller{}, f.wsID, "")
	assertDenied(t, err)
}

func TestList_OtherWorkspacesDocumentsAreNotListed(t *testing.T) {
	f := newFixture(t)
	f.doc(f.folderA, f.oaA, "x")
	otherWS, _ := testutilWorkspace(t, f)
	f.policy.grant(f.owner.NGACNodeID, f.oaA, ngac.OpRead)

	got, err := f.list(f.owner, otherWS, "")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestList_Scopes(t *testing.T) {
	f := newFixture(t)
	mine := f.doc(f.folderA, f.oaA, "Mine draft")
	active := f.doc(f.folderA, f.oaA, "Mine active")
	_, err := f.svc.Update(bg, f.owner, active.ID, texts.Change{BaseVersion: 1, Status: ptr(texts.StatusActive)})
	require.NoError(t, err)
	f.policy.grant(f.reader.NGACNodeID, f.oaA, ngac.OpRead, ngac.OpWrite)
	theirs, err := f.svc.Create(bg, f.reader, f.wsID, f.folderA, "Theirs")
	require.NoError(t, err)
	f.policy.grant(f.owner.NGACNodeID, f.oaA, ngac.OpRead)

	ids := func(scope string) []string {
		got, err := f.list(f.owner, f.wsID, scope)
		require.NoError(t, err)
		var out []string
		for _, d := range got {
			out = append(out, d.ID)
		}
		return out
	}
	assert.ElementsMatch(t, []string{mine.ID, active.ID, theirs.ID}, ids(""))
	assert.ElementsMatch(t, []string{mine.ID, active.ID}, ids("mine"))
	assert.ElementsMatch(t, []string{mine.ID}, ids("drafts"))
	assert.ElementsMatch(t, []string{theirs.ID}, ids("shared"))

	_, err = f.list(f.owner, f.wsID, "bogus")
	require.Error(t, err)
	assert.True(t, errors.Is(err, httputil.ErrInvalidInput))
}

// ---- update ----

func TestUpdate_WriteAllowsAndReadAloneDenies(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "x")
	f.policy.grant(f.reader.NGACNodeID, f.oaA, ngac.OpRead)

	_, err := f.svc.Update(bg, f.reader, d.ID, texts.Change{BaseVersion: 1, Content: ptr("<p>no</p>")})
	assertDenied(t, err)
	_, err = f.svc.Update(bg, f.other, d.ID, texts.Change{BaseVersion: 1, Content: ptr("<p>no</p>")})
	assertDenied(t, err)

	got, err := f.svc.Update(bg, f.owner, d.ID, texts.Change{BaseVersion: 1, Content: ptr("<p>yes</p>"), Title: ptr(" Mới ")})
	require.NoError(t, err)
	assert.Equal(t, "<p>yes</p>", got.Content)
	assert.Equal(t, "Mới", got.Title)
	assert.Equal(t, 2, got.Version)

	still, err := f.svc.Get(bg, f.owner, d.ID)
	require.NoError(t, err)
	assert.Equal(t, "<p>yes</p>", still.Content, "the refused saves wrote nothing")
}

func TestUpdate_StaleVersionIsAConflictAndWritesNothing(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "x")
	_, err := f.svc.Update(bg, f.owner, d.ID, texts.Change{BaseVersion: 1, Content: ptr("<p>first</p>")})
	require.NoError(t, err)

	_, err = f.svc.Update(bg, f.owner, d.ID, texts.Change{BaseVersion: 1, Content: ptr("<p>stale</p>")})

	var c *texts.ConflictError
	require.ErrorAs(t, err, &c)
	assert.True(t, errors.Is(err, texts.ErrVersionConflict))
	assert.Equal(t, 2, c.Current.Version)
	assert.Equal(t, "<p>first</p>", c.Current.Content, "the conflict carries what is stored now")
	cur, _ := f.svc.Get(bg, f.owner, d.ID)
	assert.Equal(t, "<p>first</p>", cur.Content)
	assert.Equal(t, 2, cur.Version)
}

func TestUpdate_ConflictIsStillGatedByWrite(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "x")
	_, err := f.svc.Update(bg, f.owner, d.ID, texts.Change{BaseVersion: 1, Content: ptr("<p>secret</p>")})
	require.NoError(t, err)

	// A stale save by someone with no write must be told nothing about the content.
	_, err = f.svc.Update(bg, f.other, d.ID, texts.Change{BaseVersion: 1, Content: ptr("x")})
	assertDenied(t, err)
	var c *texts.ConflictError
	assert.False(t, errors.As(err, &c))
}

func TestUpdate_RacingSavesFromOneVersionHaveOneWinner(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "x")
	f.policy.grant(f.reader.NGACNodeID, f.oaA, ngac.OpRead, ngac.OpWrite)

	const n = 8
	var wg sync.WaitGroup
	results := make(chan error, n)
	for i := 0; i < n; i++ {
		who := f.owner
		if i%2 == 1 {
			who = f.reader
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.svc.Update(bg, who, d.ID, texts.Change{BaseVersion: 1, Content: ptr("<p>" + who.UserID + "</p>")})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	won, conflicted := 0, 0
	for err := range results {
		var c *texts.ConflictError
		switch {
		case err == nil:
			won++
		case errors.As(err, &c):
			conflicted++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	assert.Equal(t, 1, won)
	assert.Equal(t, n-1, conflicted)
	cur, _ := f.svc.Get(bg, f.owner, d.ID)
	assert.Equal(t, 2, cur.Version)
}

func TestUpdate_ReturnsTheVersionItProduced(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "x")
	got, err := f.svc.Update(bg, f.owner, d.ID, texts.Change{BaseVersion: 1, Content: ptr("<p>a</p>")})
	require.NoError(t, err)
	assert.Equal(t, 2, got.Version)
	assert.Equal(t, f.owner.UserID, got.LastEditorID)
}

func TestUpdate_RejectsInvalidChanges(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "x")
	cases := map[string]texts.Change{
		"no base version": {Content: ptr("a")},
		"nothing to save": {BaseVersion: 1},
		"empty title":     {BaseVersion: 1, Title: ptr("   ")},
		"huge content":    {BaseVersion: 1, Content: ptr(strings.Repeat("a", texts.MaxContentBytes+1))},
		"NUL in content":  {BaseVersion: 1, Content: ptr("a\x00")},
		"bad utf-8":       {BaseVersion: 1, Content: ptr("a\xff")},
		"unknown status":  {BaseVersion: 1, Status: ptr("published")},
	}
	for name, ch := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := f.svc.Update(bg, f.owner, d.ID, ch)
			require.Error(t, err)
			assert.True(t, errors.Is(err, httputil.ErrInvalidInput), "err = %v", err)
		})
	}
	cur, _ := f.svc.Get(bg, f.owner, d.ID)
	assert.Equal(t, 1, cur.Version, "no invalid save moved the version")
}

// ---- delete ----

func TestDelete_NeedsWrite(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "x")
	f.policy.grant(f.reader.NGACNodeID, f.oaA, ngac.OpRead)

	assertDenied(t, f.svc.Delete(bg, f.reader, d.ID))
	assertDenied(t, f.svc.Delete(bg, f.other, d.ID))
	_, err := f.svc.Get(bg, f.owner, d.ID)
	require.NoError(t, err, "refused deletes left it in place")

	require.NoError(t, f.svc.Delete(bg, f.owner, d.ID))
	_, err = f.svc.Get(bg, f.owner, d.ID)
	assertDenied(t, err)
}
