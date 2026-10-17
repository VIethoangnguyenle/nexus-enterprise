package store

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTwoChangesOfOneShareDoNotOverlap(t *testing.T) {
	w := newWorld(t)
	var inside, overlaps atomic.Int32
	run := func(grantee string) {
		require.NoError(t, w.st.WithShareLock(context.Background(), w.cal[0], grantee, func(context.Context) error {
			if inside.Add(1) > 1 {
				overlaps.Add(1)
			}
			time.Sleep(50 * time.Millisecond)
			inside.Add(-1)
			return nil
		}))
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); run("g1") }()
	}
	wg.Wait()
	assert.Zero(t, overlaps.Load(), "the same share is changed by one request at a time")

	// Another share is not held up.
	started := make(chan struct{})
	release := make(chan struct{})
	go w.st.WithShareLock(context.Background(), w.cal[0], "g2", func(context.Context) error { close(started); <-release; return nil })
	<-started
	done := make(chan struct{})
	go func() { run("g3"); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("another share waited")
	}
	close(release)
}
