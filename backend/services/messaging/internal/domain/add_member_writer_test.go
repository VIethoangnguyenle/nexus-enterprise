package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

// The read replica can be behind a removal made a moment ago. A person removed from
// the workspace must not be put back into a group because the replica still shows
// them as a member, so AddMember asks the writer. The two views differ here on purpose:
// "stale" is a member only on the replica, "fresh" only on the writer.
func TestAddMember_AsksTheWriterWhoIsAMember(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "rt-writer-view")
	f.read.allow[grant{userNode, oaID, ngac.OpInvite}] = true
	f.read.addMember(f.wsID, "ngac-user-stale")

	view := &scriptedPolicyRead{allow: map[grant]bool{}}
	view.addMember(f.wsID, "ngac-user-fresh")
	view.addMember("another-workspace", "ngac-user-stale") // the writer has them out of this workspace
	w := testutil.NewFakePolicyWrite()
	w.Reads = view
	f.withFakeWrite(w)

	err := f.svc.AddMember(f.callerCtx(), chID, userNode, "ngac-user-stale")
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "removed on the writer, still a member on the replica")
	for _, l := range w.Log() {
		assert.False(t, strings.HasPrefix(l, "assign"), "nothing was written for the refused target: %v", w.Log())
	}

	require.NoError(t, f.svc.AddMember(f.callerCtx(), chID, userNode, "ngac-user-fresh"), "a member on the writer, unknown to the replica")
}

// When the target cannot be resolved the answer is unknown, and unknown denies:
// no assignment is written.
func TestAddMember_RefusesWhenTheTargetCannotBeResolved(t *testing.T) {
	for name, fail := range map[string]func(v *scriptedPolicyRead){
		"GetNode":      func(v *scriptedPolicyRead) { v.getNodeErr = errors.New("policy down") },
		"GetAncestors": func(v *scriptedPolicyRead) { v.ancErr = errors.New("policy down") },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			chID, oaID := f.insertChannel(t, "rt-unresolved")
			f.read.allow[grant{userNode, oaID, ngac.OpInvite}] = true
			view := &scriptedPolicyRead{allow: map[grant]bool{}}
			view.addMember(f.wsID, "ngac-user-ok")
			w := testutil.NewFakePolicyWrite()
			w.Reads = view
			f.withFakeWrite(w)

			// Allow on the same fixture, while the graph answers.
			require.NoError(t, f.svc.AddMember(f.callerCtx(), chID, userNode, "ngac-user-ok"))
			before := len(w.Log())

			fail(view)
			err := f.svc.AddMember(f.callerCtx(), chID, userNode, "ngac-user-ok-too")
			require.Error(t, err)
			assert.Len(t, w.Log(), before, "no assignment after a failed lookup")
		})
	}
}
