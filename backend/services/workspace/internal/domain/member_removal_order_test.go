package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/realtime"
)

// Once a person is detached from the workspace's UAs the access is gone, and the
// people connected to it must hear so at once. Recording the removal in the
// directory comes after, and its failure must not hold the announcement back:
// a sixty-second-old socket of the removed person would stay open for as long as
// the listing could not be written.

// tombstoneProbe notes how many directory writes had happened when each event went out.
func tombstoneProbe(f *adminFixture) *probe {
	p := &probe{written: func() int { return len(f.dir.removed) }}
	f.svc = f.svc.WithEmitter(p)
	return p
}

func removalActions() map[string]func(f *adminFixture) error {
	return map[string]func(f *adminFixture) error{
		"RemoveMember": func(f *adminFixture) error {
			f.read.grants[grant{owner, mgmt1, ngac.OpInvite}] = true
			return f.svc.RemoveMember(ctx(), owner, ws1, target)
		},
		"LeaveWorkspace": func(f *adminFixture) error { return f.svc.LeaveWorkspace(ctx(), member, ws1) },
	}
}

func TestMemberRemoval_AnnouncedBeforeTheListingIsWritten(t *testing.T) {
	for name, do := range removalActions() {
		t.Run(name, func(t *testing.T) {
			f := newAdminFixture(t)
			p := tombstoneProbe(f)

			require.NoError(t, do(f))

			require.NotEmpty(t, p.Events())
			var removedAt = -1
			for i, e := range p.Events() {
				if e.Kind == realtime.KindMemberRemoved {
					removedAt = p.seen[i]
				}
			}
			require.NotEqual(t, -1, removedAt, "member_removed was announced")
			assert.Zero(t, removedAt, "member_removed went out before the directory write, not after it")
			assert.Len(t, f.dir.removed, 1, "and the listing was marked afterwards")
		})
	}
}

func TestMemberRemoval_AListingFailureDoesNotSuppressTheAnnouncement(t *testing.T) {
	for name, do := range removalActions() {
		t.Run(name, func(t *testing.T) {
			f := newAdminFixture(t)
			p := tombstoneProbe(f)
			f.dir.markErr = errors.New("database unavailable")
			f.svc = f.svc.WithRepairBackoff(nil)

			err := do(f)

			require.Error(t, err, "the failure is reported, not swallowed")
			assert.Contains(t, err.Error(), "database unavailable")
			var sawRemoved bool
			for _, e := range p.Events() {
				if e.Kind == realtime.KindMemberRemoved {
					sawRemoved = true
				}
			}
			assert.True(t, sawRemoved, "the sockets of the removed person are still told")
			assert.NotEmpty(t, f.write.mutations, "the detach had happened")
		})
	}
}

// A refused removal announces nothing and marks nothing: the failure path above
// must not be reachable by a caller who may not remove anyone.
func TestMemberRemoval_RefusedWritesAndAnnouncesNothing(t *testing.T) {
	f := newAdminFixture(t)
	p := tombstoneProbe(f)

	err := f.svc.RemoveMember(ctx(), outsider, ws1, target)

	require.Error(t, err)
	assert.Empty(t, p.Events())
	assert.Empty(t, f.dir.removed)
	assert.False(t, f.mutated())
}

// A listing that could not be written is not left active for ever: the person is out of the
// graph, and the rest of the product reads the row. The write is tried again in the
// background, on a schedule that ends, and the caller has already been told it failed.
func TestMemberRemoval_AFailedListingIsRepairedInTheBackground(t *testing.T) {
	for name, do := range removalActions() {
		t.Run(name, func(t *testing.T) {
			f := newAdminFixture(t)
			f.dir.failFirst = 2
			f.svc = f.svc.WithRepairBackoff([]time.Duration{5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond})

			err := do(f)

			require.Error(t, err, "the first attempt failed and the caller is told")
			require.Eventually(t, func() bool { return len(f.dir.marked()) == 1 }, 2*time.Second, 5*time.Millisecond,
				"the listing was marked removed by the repair")
		})
	}
}

func TestMemberRemoval_TheRepairGivesUpWhenItsScheduleEnds(t *testing.T) {
	f := newAdminFixture(t)
	f.dir.failFirst = 1000
	f.svc = f.svc.WithRepairBackoff([]time.Duration{5 * time.Millisecond, 5 * time.Millisecond})

	require.Error(t, removalActions()["LeaveWorkspace"](f))

	require.Eventually(t, func() bool { f.dir.mu.Lock(); defer f.dir.mu.Unlock(); return f.dir.attempts == 3 }, 2*time.Second, 5*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	f.dir.mu.Lock()
	defer f.dir.mu.Unlock()
	assert.Equal(t, 3, f.dir.attempts, "one attempt in the request and one per step of the schedule, no more")
}

// The paired allow: when the first write works there is nothing to repair.
func TestMemberRemoval_ASuccessfulListingStartsNoRepair(t *testing.T) {
	f := newAdminFixture(t)
	f.svc = f.svc.WithRepairBackoff([]time.Duration{5 * time.Millisecond})

	require.NoError(t, removalActions()["LeaveWorkspace"](f))
	time.Sleep(60 * time.Millisecond)

	f.dir.mu.Lock()
	defer f.dir.mu.Unlock()
	assert.Equal(t, 1, f.dir.attempts)
}
