package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/store"
	"ngac-platform/testutil"
)

// searchWorld is one tenant schema with requests from two requesters, each a
// real account (user ID and NGAC node). Every request is the caller's own, so
// the access filter lets all of them through and only the filter under test
// decides.
type searchWorld struct {
	st                 *store.Store
	ctx                context.Context
	template           string
	aliceID, aliceNode string
	bobID, bobNode     string
}

func newSearchWorld(t *testing.T) *searchWorld {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	id := "srch-" + uuid.NewString()
	newOrg(t, pool, id)
	schema, err := st.EnsureSchema(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	w := &searchWorld{st: st, ctx: httputil.WithTenantSchema(context.Background(), schema)}
	w.aliceID, w.aliceNode = testutil.CreateUser(t, pool)
	w.bobID, w.bobNode = testutil.CreateUser(t, pool)
	w.template = insertTemplate(t, w.ctx, st)
	return w
}

func (w *searchWorld) request(t *testing.T, templateName, createdBy string) string {
	t.Helper()
	id := uuid.NewString()
	if err := w.st.InsertRequest(w.ctx, &domain.Request{
		ID: id, EntityType: "leave", EntityID: uuid.NewString(), TemplateID: w.template, TemplateName: templateName,
		TemplateSnapshot: `{}`, CurrentStep: 1, Status: "pending", ScopeOAID: "oa-" + id, DepartmentID: "dept",
		CreatedBy: createdBy, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func (w *searchWorld) search(t *testing.T, as string, p domain.SearchParams) []string {
	t.Helper()
	p.UserNodeID, p.Limit = as, 50
	items, _, err := w.st.SearchRequests(w.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range items {
		ids = append(ids, r.ID)
	}
	return ids
}

// The "Người gửi" filter names the requester by their user ID, which is what the
// screen has; the store resolves it to the node a request records.
func TestSearchRequests_CreatedByIsTheRequestersUserID(t *testing.T) {
	w := newSearchWorld(t)
	alices := w.request(t, "Nghỉ phép", w.aliceNode)
	w.request(t, "Nghỉ phép", w.bobNode)

	got := w.search(t, w.aliceNode, domain.SearchParams{Query: "nghỉ", CreatedBy: w.aliceID})
	if len(got) != 1 || got[0] != alices {
		t.Fatalf("created_by = Alice's user ID found %v, want [%s]", got, alices)
	}

	// Deny: someone else's user ID, an unknown one, and a node ID passed as a
	// user ID each match nothing of Alice's.
	for name, by := range map[string]string{"another requester": w.bobID, "unknown": "no-such-user", "a node ID": w.aliceNode} {
		if got := w.search(t, w.aliceNode, domain.SearchParams{Query: "nghỉ", CreatedBy: by}); len(got) != 0 {
			t.Errorf("%s: found %v, want nothing", name, got)
		}
	}
}

// % and _ in the query are the characters themselves, not LIKE wildcards: "%%"
// passes the length rule but matches only a name that holds "%%".
func TestSearchRequests_QueryWildcardsAreLiteral(t *testing.T) {
	w := newSearchWorld(t)
	w.request(t, "Nghỉ phép", w.aliceNode)
	w.request(t, "Mua sắm", w.aliceNode)
	percent := w.request(t, "Giảm 50%% giá", w.aliceNode)
	underscore := w.request(t, "mã_kho", w.aliceNode)

	if got := w.search(t, w.aliceNode, domain.SearchParams{Query: "%%"}); len(got) != 1 || got[0] != percent {
		t.Errorf(`q="%%%%" found %v, want only the name holding "%%%%"`, got)
	}
	if got := w.search(t, w.aliceNode, domain.SearchParams{Query: "__"}); len(got) != 0 {
		t.Errorf(`q="__" found %v, want nothing`, got)
	}
	if got := w.search(t, w.aliceNode, domain.SearchParams{Query: "ã_k"}); len(got) != 1 || got[0] != underscore {
		t.Errorf(`q="ã_k" found %v, want [%s]`, got, underscore)
	}
	if got := w.search(t, w.aliceNode, domain.SearchParams{Query: `\%`}); len(got) != 0 {
		t.Errorf(`q="\%%" found %v, want nothing`, got)
	}
}
