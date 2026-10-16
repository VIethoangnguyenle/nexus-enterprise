package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"ngac-platform/pkg/imagenorm"
	"ngac-platform/services/auth/internal/store"
)

const (
	// AvatarMaxBytes is the largest file a person may upload as their picture.
	AvatarMaxBytes = 5 << 20
	// AvatarURLPrefix is where the pictures are served from; a profile's
	// avatar_url is this followed by the picture's token.
	AvatarURLPrefix = "/api/auth/avatars/"
	// AvatarUploadLimit is how many pictures one person may upload per hour.
	// Each upload is a decode and three encodes in the process that also signs
	// everyone in, so a loop of them is a way to slow sign-in down.
	AvatarUploadLimit = 10

	avatarUploadWindow = time.Hour
	avatarRateKey      = "avatar_rl:"

	// avatarQuality is the JPEG quality of every stored size.
	avatarQuality = 85
	// avatarBusyWait is how long an upload waits for the one before it.
	avatarBusyWait = 5 * time.Second
)

// The three sizes a picture is kept in: lists of small avatars, the larger
// avatars of panels and calls, and the detail view.
const (
	avatarSmall  = 64
	avatarMedium = 192
	avatarLarge  = 1024
)

var (
	// ErrAvatarNotImage means the upload is not a JPEG or PNG picture,
	// whatever its name or declared type says.
	ErrAvatarNotImage = errors.New("the file is not a JPEG or PNG picture")
	// ErrAvatarTooManyPixels means the picture is too large to decode safely.
	ErrAvatarTooManyPixels = errors.New("the picture has too many pixels")
	// ErrAvatarBusy means another picture is being processed and did not
	// finish in time.
	ErrAvatarBusy = errors.New("another picture is being processed, try again shortly")
)

var avatarToken = regexp.MustCompile(`^[0-9a-f]{32}$`)

// SetAvatar makes b the person's picture and returns the URL their profile now
// points at. The picture is re-encoded, so nothing of the original file but its
// pixels is kept: no EXIF, no location, no other metadata. Each upload gets a
// new random token, so the URL says nothing about the picture.
func (s *Service) SetAvatar(ctx context.Context, userID string, b []byte) (string, error) {
	if userID == "" {
		return "", ErrInvalidInput
	}
	if err := s.takeAvatarUpload(ctx, userID); err != nil {
		return "", err
	}
	release, err := s.acquireAvatarGate(ctx)
	if err != nil {
		return "", err
	}
	// Deferred, so a decoder panic (caught by the HTTP recovery middleware)
	// cannot keep the one slot taken and wedge every later upload.
	img, err := func() (store.AvatarImages, error) {
		defer release()
		return normalizeAvatar(b)
	}()
	if err != nil {
		return "", err
	}

	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("avatar token: %w", err)
	}
	token := hex.EncodeToString(raw)
	url := AvatarURLPrefix + token
	if err := s.store.SetAvatar(ctx, userID, token, img, url); err != nil {
		if errors.Is(err, store.ErrNoSuchUser) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("set avatar: %w", err)
	}
	return url, nil
}

// ClearAvatar removes the person's picture; their initials show again.
func (s *Service) ClearAvatar(ctx context.Context, userID string) error {
	if userID == "" {
		return ErrInvalidInput
	}
	if err := s.store.ClearAvatar(ctx, userID); err != nil {
		if errors.Is(err, store.ErrNoSuchUser) {
			return ErrNotFound
		}
		return fmt.Errorf("clear avatar: %w", err)
	}
	return nil
}

// Avatar returns one size (64, 192 or 1024) of the picture under token, as a
// JPEG. Anything that names no stored picture is ErrNotFound.
func (s *Service) Avatar(ctx context.Context, token string, size int) ([]byte, error) {
	if !avatarToken.MatchString(token) {
		return nil, ErrNotFound
	}
	switch size {
	case avatarSmall, avatarMedium, avatarLarge:
	default:
		return nil, ErrNotFound
	}
	b, err := s.store.GetAvatar(ctx, token, size)
	if err != nil {
		return nil, fmt.Errorf("get avatar: %w", err)
	}
	if b == nil {
		return nil, ErrNotFound
	}
	return b, nil
}

// takeAvatarUpload spends one of the person's uploads for this hour. Without
// Redis (only unit tests run that way) nothing is counted.
func (s *Service) takeAvatarUpload(ctx context.Context, userID string) error {
	if s.rdb == nil {
		return nil
	}
	key := avatarRateKey + userID
	n, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("avatar rate limit: %w", err)
	}
	if n == 1 {
		s.rdb.Expire(ctx, key, avatarUploadWindow)
	}
	if n <= AvatarUploadLimit {
		return nil
	}
	return &rateLimitedError{base: ErrRateLimited, after: s.windowLeft(ctx, key, avatarUploadWindow)}
}

// acquireAvatarGate lets one decode run at a time in this process, so two
// large pictures never take twice the budget. A caller waits up to
// avatarBusyWait for the one before it.
func (s *Service) acquireAvatarGate(ctx context.Context) (func(), error) {
	t := time.NewTimer(s.avatarWait)
	defer t.Stop()
	select {
	case s.avatarGate <- struct{}{}:
		return func() { <-s.avatarGate }, nil
	case <-t.C:
		return nil, ErrAvatarBusy
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// normalizeAvatar turns an uploaded JPEG or PNG into the three stored sizes:
// the centre square of the picture, the right way up, on white where it was
// transparent, as JPEG.
func normalizeAvatar(b []byte) (store.AvatarImages, error) {
	out, err := imagenorm.Square(b, []int{avatarLarge, avatarMedium, avatarSmall}, avatarQuality)
	switch {
	case errors.Is(err, imagenorm.ErrNotImage):
		return store.AvatarImages{}, ErrAvatarNotImage
	case errors.Is(err, imagenorm.ErrTooManyPixels):
		return store.AvatarImages{}, ErrAvatarTooManyPixels
	case err != nil:
		return store.AvatarImages{}, fmt.Errorf("normalize avatar: %w", err)
	}
	return store.AvatarImages{Large: out[0], Medium: out[1], Small: out[2]}, nil
}
