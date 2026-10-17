package domain

import (
	"testing"

	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/calendar/internal/policyfake"
	"ngac-platform/services/calendar/internal/store"
)

// A person named in a request (a guest, a grantee) must be a member now. The read
// replica can lag a removal, so the writer decides: "n-bob" is a member only on the
// replica (the writer has already removed him), "n-fresh" only on the writer.
func TestRequirePersonAsksTheWriter(t *testing.T) {
	f := newFixture(t)
	f.st.people["n-fresh"] = &store.Person{NodeID: "n-fresh", DisplayName: "Fresh"}
	f.w.Seed("n-fresh", ngac.UserNodeName("n-fresh"), "U", nil) // known to the replica, in no workspace

	wr := policyfake.NewWorld()
	wr.Seed("pc1", ngac.PCName(ws), "PC", nil)
	wr.Seed("ua-members", ngac.MembersUAName(ws), "UA", nil)
	wr.Assign("ua-members", "pc1")
	wr.Seed("n-bob", ngac.UserNodeName("n-bob"), "U", nil) // removed: under nothing
	wr.Seed("n-fresh", ngac.UserNodeName("n-fresh"), "U", nil)
	wr.Assign("n-fresh", "ua-members")
	f.svc.policyWrite = wr.Write()

	_, err := f.svc.requirePerson(callerCtx("n-alice"), f.st.workspace, "n-bob")
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.requirePerson(callerCtx("n-alice"), f.st.workspace, "n-fresh")
	require.NoError(t, err)
}
