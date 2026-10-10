package store_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

func TestWithOwnerLock_SerialisesPerWorkspace(t *testing.T) {
	st := store.New(testutil.SetupTestDB(t))
	var inside, overlaps int32
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := st.WithOwnerLock(context.Background(), "ws-lock-a", func(context.Context) error {
				if atomic.AddInt32(&inside, 1) > 1 {
					atomic.AddInt32(&overlaps, 1)
				}
				time.Sleep(15 * time.Millisecond)
				atomic.AddInt32(&inside, -1)
				return nil
			})
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	assert.Zero(t, overlaps, "two owner changes ran at once in one workspace")
}

func TestWithOwnerLock_DifferentWorkspacesDoNotWait(t *testing.T) {
	st := store.New(testutil.SetupTestDB(t))
	release := make(chan struct{})
	held := make(chan struct{})
	go func() {
		_ = st.WithOwnerLock(context.Background(), "ws-lock-b", func(context.Context) error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	done := make(chan error, 1)
	go func() {
		done <- st.WithOwnerLock(context.Background(), "ws-lock-c", func(context.Context) error { return nil })
	}()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("another workspace's lock was blocked")
	}
	close(release)
}

func TestWithOwnerLock_ReleasesOnErrorAndReturnsIt(t *testing.T) {
	st := store.New(testutil.SetupTestDB(t))
	boom := errors.New("boom")
	require.ErrorIs(t, st.WithOwnerLock(context.Background(), "ws-lock-d", func(context.Context) error { return boom }), boom)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, st.WithOwnerLock(ctx, "ws-lock-d", func(context.Context) error { return nil }), "the lock was released")
}
