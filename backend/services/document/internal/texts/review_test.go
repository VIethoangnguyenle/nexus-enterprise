package texts_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	"ngac-platform/services/document/internal/texts"
)

// ---- writing is not reading ----

func TestUpdate_WriteWithoutReadSavesButIsShownNothing(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "Bí mật")
	_, err := f.svc.Update(bg, f.owner, d.ID, texts.Change{BaseVersion: 1, Content: ptr("<p>nội dung kín</p>")})
	require.NoError(t, err)
	f.policy.grant(f.reader.NGACNodeID, f.oaA, ngac.OpWrite) // write only

	got, err := f.svc.Update(bg, f.reader, d.ID, texts.Change{BaseVersion: 2, Content: ptr("<p>của tôi</p>")})

	require.NoError(t, err)
	assert.False(t, got.CanRead)
	assert.Empty(t, got.Content, "the saved text is not echoed to someone who may not read it")
	assert.Empty(t, got.Title)
	assert.Equal(t, 3, got.Version)
}

func TestUpdate_ConflictShowsAWriteOnlyCallerTheVersionAndNothingElse(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "Bí mật")
	_, err := f.svc.Update(bg, f.owner, d.ID, texts.Change{BaseVersion: 1, Content: ptr("<p>nội dung kín</p>")})
	require.NoError(t, err)
	f.policy.grant(f.reader.NGACNodeID, f.oaA, ngac.OpWrite)

	_, err = f.svc.Update(bg, f.reader, d.ID, texts.Change{BaseVersion: 1, Content: ptr("x")})

	var c *texts.ConflictError
	require.ErrorAs(t, err, &c)
	assert.False(t, c.Readable)
	assert.Equal(t, 2, c.Current.Version)
	assert.Empty(t, c.Current.Content)
	assert.Empty(t, c.Current.Title)
	assert.Empty(t, c.Current.OwnerID)
}

func TestUpdate_ConflictShowsAReaderTheCurrentText(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "x")
	_, err := f.svc.Update(bg, f.owner, d.ID, texts.Change{BaseVersion: 1, Content: ptr("<p>mới</p>")})
	require.NoError(t, err)

	_, err = f.svc.Update(bg, f.owner, d.ID, texts.Change{BaseVersion: 1, Content: ptr("<p>cũ</p>")})

	var c *texts.ConflictError
	require.ErrorAs(t, err, &c)
	assert.True(t, c.Readable)
	assert.Equal(t, "<p>mới</p>", c.Current.Content)
}

func TestWire_WriteOnlyCallerNeverGetsTextInAnyResponse(t *testing.T) {
	w := newWire(t)
	f := w.f
	d := f.doc(f.folderA, f.oaA, "Tiêu đề kín")
	_, err := f.svc.Update(bg, f.owner, d.ID, texts.Change{BaseVersion: 1, Content: ptr("<p>nội dung kín</p>")})
	require.NoError(t, err)
	f.policy.grant(f.reader.NGACNodeID, f.oaA, ngac.OpWrite)
	path := "/api/documents/texts/" + d.ID

	code, ok := w.do(&f.reader, "PATCH", path, `{"base_version":2,"content":"<p>của tôi</p>"}`)
	require.Equal(t, http.StatusOK, code)
	assert.NotContains(t, ok, "content")
	assert.EqualValues(t, 3, ok["version"])

	code, conflict := w.do(&f.reader, "PATCH", path, `{"base_version":1,"content":"x"}`)
	require.Equal(t, http.StatusConflict, code)
	assert.Equal(t, map[string]any{"version": float64(3)}, conflict["current"])

	code, _ = w.do(&f.reader, "GET", path, "")
	assert.Equal(t, http.StatusForbidden, code, "the same caller cannot read it the plain way either")
}

// ---- stored text is clean ----

func TestSanitize_KeepsWhatTheEditorMakesAndDropsTheRest(t *testing.T) {
	in := `<h2>Tiêu đề</h2><p>Một <strong>đậm</strong>, <em>nghiêng</em>, <u>gạch</u>, <s>ngang</s> và <a href="https://example.test/a">link</a> <a href="mailto:a@b.test">mail</a></p><ul><li>a</li></ul><ol><li>b</li></ol><blockquote><p>q</p></blockquote><pre><code>x</code></pre><hr>`
	out := texts.Sanitize(in)
	for _, keep := range []string{"<h2>Tiêu đề</h2>", "<strong>đậm</strong>", "<em>nghiêng</em>", "<u>gạch</u>", "<s>ngang</s>", "<ul><li>a</li></ul>", "<ol><li>b</li></ol>", "<blockquote>", "<pre><code>x</code></pre>", `href="https://example.test/a"`, `href="mailto:a@b.test"`} {
		assert.Contains(t, out, keep)
	}
}

func TestSanitize_DropsPayloads(t *testing.T) {
	cases := map[string]string{
		"script":            `<p>a</p><script>alert(1)</script>`,
		"onerror":           `<img src=x onerror="alert(1)">`,
		"onclick":           `<p onclick="alert(1)">a</p>`,
		"javascript link":   `<a href="javascript:alert(1)">x</a>`,
		"data link":         `<a href="data:text/html;base64,PHNjcmlwdD4=">x</a>`,
		"iframe":            `<iframe src="https://e.test"></iframe>`,
		"style attribute":   `<p style="background:url(javascript:alert(1))">a</p>`,
		"style tag":         `<style>p{}</style><p>a</p>`,
		"object":            `<object data="x"></object><embed src="x">`,
		"form":              `<form action="https://e.test"><input name=a></form>`,
		"svg":               `<svg onload="alert(1)"><script>1</script></svg>`,
		"mixed case scheme": `<a href="JaVaScRiPt:alert(1)">x</a>`,
		"entity scheme":     `<a href="jav&#x61;script:alert(1)">x</a>`,
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			out := texts.Sanitize(in)
			assert.NotRegexp(t, `(?i)<script|<iframe|<object|<embed|<form|<input|<svg|<style|onerror|onclick|onload|javascript:|data:|style=`, out)
		})
	}
}

func TestSanitize_AClassOrIdDoesNotSurvive(t *testing.T) {
	assert.Equal(t, "<p>a</p>", texts.Sanitize(`<p class="x" id="y" data-k="v">a</p>`))
}

func TestUpdate_StoresTheSanitizedText(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "x")

	got, err := f.svc.Update(bg, f.owner, d.ID, texts.Change{BaseVersion: 1,
		Content: ptr(`<p onclick="x()">an toàn</p><script>alert(1)</script><a href="javascript:alert(1)">l</a>`)})

	require.NoError(t, err)
	assert.NotRegexp(t, `(?i)script|onclick|javascript:`, got.Content)
	assert.Contains(t, got.Content, "an toàn")
	stored, err := f.svc.Get(bg, f.owner, d.ID)
	require.NoError(t, err)
	assert.Equal(t, got.Content, stored.Content, "what is stored is what was returned")
}

// ---- a document deleted while a save is in flight ----

func TestUpdate_DeletedMidSaveAnswersLikeDenied(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "x")
	// After the write check passes, the document goes (the read check is next).
	f.policy.onCheck = func(_, _, op string) {
		if op == ngac.OpRead {
			f.exec(`DELETE FROM text_documents WHERE id = $1`, d.ID)
		}
	}

	_, err := f.svc.Update(bg, f.owner, d.ID, texts.Change{BaseVersion: 1, Content: ptr("<p>x</p>")})

	require.Error(t, err)
	assert.True(t, errors.Is(err, httputil.ErrAccessDenied), "err = %v", err)
}

// ---- paging ----

func (f *fixture) docAt(folder, oa, title string, at time.Time) *texts.Doc {
	d := f.doc(folder, oa, title)
	f.exec(`UPDATE text_documents SET updated_at = $2 WHERE id = $1`, d.ID, at)
	return d
}

func TestList_PagesWithACursorAndNothingRepeatsOrIsMissed(t *testing.T) {
	f := newFixture(t)
	base := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	var want []string
	for i := 0; i < 7; i++ {
		d := f.docAt(f.folderA, f.oaA, fmt.Sprintf("Doc %d", i), base.Add(time.Duration(i)*time.Minute))
		want = append([]string{d.ID}, want...) // newest first
	}

	var got []string
	cursor := ""
	for pages := 0; pages < 10; pages++ {
		page, err := f.svc.List(bg, f.owner, f.wsID, "", cursor, 3)
		require.NoError(t, err)
		require.LessOrEqual(t, len(page.Docs), 3)
		for _, d := range page.Docs {
			got = append(got, d.ID)
		}
		if page.Next == "" {
			break
		}
		cursor = page.Next
	}
	assert.Equal(t, want, got)
}

func TestList_APageThatEndsExactlyOnTheLastRowHasNoNext(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 4; i++ {
		f.doc(f.folderA, f.oaA, fmt.Sprintf("D%d", i))
	}
	page, err := f.svc.List(bg, f.owner, f.wsID, "", "", 4)
	require.NoError(t, err)
	assert.Len(t, page.Docs, 4)
	if page.Next != "" {
		next, err := f.svc.List(bg, f.owner, f.wsID, "", page.Next, 4)
		require.NoError(t, err)
		assert.Empty(t, next.Docs, "a Next that leads to nothing is allowed; one that leads to more rows is not")
	}
}

func TestList_PagingNeverShowsWhatTheCallerMayNotRead(t *testing.T) {
	f := newFixture(t)
	base := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	f.policy.grant(f.reader.NGACNodeID, f.oaA, ngac.OpRead)
	var readable []string
	for i := 0; i < 6; i++ { // A and B interleaved; the reader may read A only
		oa, folder := f.oaA, f.folderA
		if i%2 == 1 {
			oa, folder = f.oaB, f.folderB
		}
		d := f.docAt(folder, oa, fmt.Sprintf("D%d", i), base.Add(time.Duration(i)*time.Minute))
		if oa == f.oaA {
			readable = append([]string{d.ID}, readable...)
		}
	}
	var got []string
	cursor := ""
	for pages := 0; pages < 10; pages++ {
		page, err := f.svc.List(bg, f.reader, f.wsID, "", cursor, 2)
		require.NoError(t, err)
		for _, d := range page.Docs {
			got = append(got, d.ID)
		}
		if page.Next == "" {
			break
		}
		cursor = page.Next
	}
	assert.Equal(t, readable, got)
}

func TestList_HardCapAndBadCursor(t *testing.T) {
	f := newFixture(t)
	f.doc(f.folderA, f.oaA, "x")
	page, err := f.svc.List(bg, f.owner, f.wsID, "", "", 100000)
	require.NoError(t, err, "an oversized limit is clamped, not refused")
	assert.LessOrEqual(t, len(page.Docs), texts.MaxPageSize)

	for _, bad := range []string{"%%%", "bm90LWEtY3Vyc29y", "MXw"} {
		_, err := f.svc.List(bg, f.owner, f.wsID, "", bad, 10)
		require.Error(t, err, bad)
		assert.True(t, errors.Is(err, httputil.ErrInvalidInput), "cursor %q: %v", bad, err)
	}
}

func TestList_APolicyOutageMidPageListsNothing(t *testing.T) {
	f := newFixture(t)
	f.doc(f.folderA, f.oaA, "x")
	f.policy.fail = errors.New("down")
	_, err := f.svc.List(bg, f.owner, f.wsID, "", "", 10)
	require.Error(t, err)
}

// ---- the badge ----

func TestCount_CountsOnlyWhatIsReadable(t *testing.T) {
	f := newFixture(t)
	f.doc(f.folderA, f.oaA, "a1")
	f.doc(f.folderA, f.oaA, "a2")
	f.doc(f.folderB, f.oaB, "b1")
	f.policy.grant(f.reader.NGACNodeID, f.oaA, ngac.OpRead)

	n, err := f.svc.Count(bg, f.reader, f.wsID, "")
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	n, err = f.svc.Count(bg, f.other, f.wsID, "")
	require.NoError(t, err)
	assert.Zero(t, n)

	n, err = f.svc.Count(bg, f.owner, f.wsID, "drafts")
	require.NoError(t, err)
	assert.Equal(t, 3, n, "the owner's own drafts, all readable by them")
}

func TestCount_FailsClosed(t *testing.T) {
	f := newFixture(t)
	f.doc(f.folderA, f.oaA, "a")
	_, err := f.svc.Count(bg, texts.Caller{}, f.wsID, "")
	assertDenied(t, err)
	_, err = f.svc.Count(bg, f.owner, f.wsID, "bogus")
	require.Error(t, err)
	f.policy.fail = errors.New("down")
	_, err = f.svc.Count(bg, f.owner, f.wsID, "")
	require.Error(t, err)
}

// ---- what outlives what ----

func TestLifecycle_ADocumentOutlivesItsAuthorAndIsNotLostWithItsFolder(t *testing.T) {
	f := newFixture(t)
	d := f.doc(f.folderA, f.oaA, "Còn mãi")
	f.policy.grant(f.reader.NGACNodeID, f.oaA, ngac.OpRead)

	// Hard-deleting the folder must not take the writing with it.
	_, err := f.pool.Exec(context.Background(), `DELETE FROM drive_items WHERE id = $1`, f.folderA)
	require.Error(t, err, "the folder cannot be removed while documents sit in it")
	got, err := f.svc.Get(bg, f.reader, d.ID)
	require.NoError(t, err)
	assert.Equal(t, "Còn mãi", got.Title)

	// Deleting the author leaves an ownerless document, shown as nobody.
	f.exec(`DELETE FROM tenant_users WHERE user_id = $1`, f.owner.UserID)
	f.exec(`UPDATE workspaces SET owner_id = $1 WHERE id = $2`, f.reader.UserID, f.wsID)
	f.exec(`DELETE FROM users WHERE id = $1`, f.owner.UserID)

	got, err = f.svc.Get(bg, f.reader, d.ID)
	require.NoError(t, err)
	assert.Empty(t, got.OwnerID)
	assert.Empty(t, got.OwnerName)
	docs, err := f.list(f.reader, f.wsID, "shared")
	require.NoError(t, err)
	assert.Len(t, docs, 1, "a document with no author is someone else's, so it is shared")
}
