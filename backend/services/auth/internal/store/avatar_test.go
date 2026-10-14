package store_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/auth/internal/store"
)

func avatarImages(tag string) store.AvatarImages {
	return store.AvatarImages{Small: []byte("s-" + tag), Medium: []byte("m-" + tag), Large: []byte("l-" + tag)}
}

func (f *profileFixture) avatarURL(t *testing.T, id string) string {
	t.Helper()
	var url string
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT COALESCE(avatar_url,'') FROM users WHERE id = $1`, id).Scan(&url))
	return url
}

func (f *profileFixture) avatarRows(t *testing.T, id string) int {
	t.Helper()
	var n int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM user_avatars WHERE user_id = $1`, id).Scan(&n))
	return n
}

func TestSetAvatar_StoresEverySizeAndPointsTheProfileAtIt(t *testing.T) {
	f := newProfileFixture(t)
	id := f.user(t, "av", "")
	tok := fmt.Sprintf("%032x", f.n)

	require.NoError(t, f.st.SetAvatar(f.ctx, id, tok, avatarImages("a"), "/api/auth/avatars/"+tok))
	assert.Equal(t, "/api/auth/avatars/"+tok, f.avatarURL(t, id))
	for size, want := range map[int]string{64: "s-a", 192: "m-a", 1024: "l-a"} {
		b, err := f.st.GetAvatar(f.ctx, tok, size)
		require.NoError(t, err)
		assert.Equal(t, want, string(b), "size %d", size)
	}
}

func TestSetAvatar_ReplacingRetiresTheOldToken(t *testing.T) {
	f := newProfileFixture(t)
	id := f.user(t, "avr", "")
	old, next := fmt.Sprintf("%032x", f.n), fmt.Sprintf("%032x", f.n+1)

	require.NoError(t, f.st.SetAvatar(f.ctx, id, old, avatarImages("old"), "/api/auth/avatars/"+old))
	require.NoError(t, f.st.SetAvatar(f.ctx, id, next, avatarImages("new"), "/api/auth/avatars/"+next))

	b, err := f.st.GetAvatar(f.ctx, old, 192)
	require.NoError(t, err)
	assert.Nil(t, b, "a replaced picture is not served")
	b, err = f.st.GetAvatar(f.ctx, next, 192)
	require.NoError(t, err)
	assert.Equal(t, "m-new", string(b))
	assert.Equal(t, 1, f.avatarRows(t, id), "one picture per person")
}

func TestClearAvatar_RemovesThePictureAndTheLink(t *testing.T) {
	f := newProfileFixture(t)
	id := f.user(t, "avc", "")
	tok := fmt.Sprintf("%032x", f.n)
	require.NoError(t, f.st.SetAvatar(f.ctx, id, tok, avatarImages("a"), "/api/auth/avatars/"+tok))

	require.NoError(t, f.st.ClearAvatar(f.ctx, id))
	assert.Equal(t, "", f.avatarURL(t, id))
	assert.Equal(t, 0, f.avatarRows(t, id))
	require.NoError(t, f.st.ClearAvatar(f.ctx, id), "clearing when there is nothing to clear is not an error")
}

func TestAvatar_UnknownUserIsRefusedAndWritesNothing(t *testing.T) {
	f := newProfileFixture(t)
	tok := fmt.Sprintf("%032x", f.n)

	assert.ErrorIs(t, f.st.SetAvatar(f.ctx, "no-such-user", tok, avatarImages("x"), "/api/auth/avatars/"+tok), store.ErrNoSuchUser)
	assert.ErrorIs(t, f.st.ClearAvatar(f.ctx, "no-such-user"), store.ErrNoSuchUser)
	b, err := f.st.GetAvatar(f.ctx, tok, 192)
	require.NoError(t, err)
	assert.Nil(t, b)
}

func TestGetAvatar_UnknownTokenOrSizeIsNothing(t *testing.T) {
	f := newProfileFixture(t)
	id := f.user(t, "avu", "")
	tok := fmt.Sprintf("%032x", f.n)
	require.NoError(t, f.st.SetAvatar(f.ctx, id, tok, avatarImages("a"), "/api/auth/avatars/"+tok))

	for _, c := range []struct {
		token string
		size  int
	}{{fmt.Sprintf("%032x", f.n+99), 192}, {tok, 100}, {tok, 0}} {
		b, err := f.st.GetAvatar(f.ctx, c.token, c.size)
		require.NoError(t, err)
		assert.Nil(t, b, "token %s size %d", c.token, c.size)
	}
}

func TestAvatar_GoesWithTheAccount(t *testing.T) {
	f := newProfileFixture(t)
	id := f.user(t, "avd", "")
	tok := fmt.Sprintf("%032x", f.n)
	require.NoError(t, f.st.SetAvatar(f.ctx, id, tok, avatarImages("a"), "/api/auth/avatars/"+tok))

	_, err := f.pool.Exec(f.ctx, `DELETE FROM users WHERE id = $1`, id)
	require.NoError(t, err)
	assert.Equal(t, 0, f.avatarRows(t, id))
}

// Two tabs changing and removing the picture at once must neither deadlock nor
// leave the profile pointing at a picture that is gone (or the reverse).
func TestAvatar_ConcurrentSetAndClearStayConsistent(t *testing.T) {
	f := newProfileFixture(t)
	id := f.user(t, "avx", "")

	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := 0; i < 20; i++ {
		wg.Add(2)
		tok := fmt.Sprintf("%016x%016x", f.n, i)
		go func() {
			defer wg.Done()
			errs <- f.st.SetAvatar(context.Background(), id, tok, avatarImages("c"), "/api/auth/avatars/"+tok)
		}()
		go func() {
			defer wg.Done()
			errs <- f.st.ClearAvatar(context.Background(), id)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	url, rows := f.avatarURL(t, id), f.avatarRows(t, id)
	if url == "" {
		assert.Equal(t, 0, rows, "no link, so no picture")
		return
	}
	require.Equal(t, 1, rows)
	var tok string
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT token FROM user_avatars WHERE user_id = $1`, id).Scan(&tok))
	assert.Equal(t, "/api/auth/avatars/"+tok, url, "the link names the picture that is kept")
}
