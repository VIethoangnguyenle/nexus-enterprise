package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"ngac-platform/ngac"
	"ngac-platform/pkg/imagenorm"
	"ngac-platform/pkg/realtime"
	"ngac-platform/services/messaging/internal/store"
)

const (
	// AvatarMaxBytes is the largest file that may be uploaded as a group picture.
	AvatarMaxBytes = 5 << 20
	// AvatarURLPrefix is where the pictures are served from; a channel's
	// avatar_url is this followed by the picture's token.
	AvatarURLPrefix = "/api/channels/avatars/"
	// AvatarUploadLimit is how many pictures one person may upload per hour.
	// Each upload is a decode and three encodes in a process that also carries
	// everyone's chat, so a loop of them is a way to slow messaging down.
	AvatarUploadLimit = 10
	// AvatarPublicLimit is how many pictures one address may fetch per window.
	// A conversation list shows dozens at once; this stops a script walking
	// addresses, not a busy page (browsers keep a picture for a year).
	AvatarPublicLimit  = 1200
	AvatarPublicWindow = time.Minute

	avatarUploadWindow = time.Hour
	avatarRateKey      = "channel_avatar_rl:"
	avatarPublicKey    = "channel_avatar_pub_rl:"

	// avatarQuality is the JPEG quality of every stored size.
	avatarQuality = 85
	// avatarBusyWait is how long an upload waits for the one before it.
	avatarBusyWait = 5 * time.Second
)

// The three sizes a picture is kept in: lists of small avatars, the larger
// avatars of panels, and the detail view.
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
	// ErrRateLimited means an allowance is used up; RetryAfter says when to
	// come back.
	ErrRateLimited = errors.New("too many requests, try again later")
)

// rateLimitedError wraps ErrRateLimited with how long the caller has to wait.
type rateLimitedError struct{ after time.Duration }

func (e *rateLimitedError) Error() string { return ErrRateLimited.Error() }
func (e *rateLimitedError) Unwrap() error { return ErrRateLimited }

// RetryAfter reports how long a rate-limited caller has to wait, when known.
func RetryAfter(err error) (time.Duration, bool) {
	var r *rateLimitedError
	if errors.As(err, &r) && r.after > 0 {
		return r.after, true
	}
	return 0, false
}

var avatarToken = regexp.MustCompile(`^[0-9a-f]{32}$`)

// authorizeAvatarChange loads the group and checks the caller may change what
// every member sees of it: manage on its content OA, the check a rename takes.
// A direct message has no picture of its own.
func (s *Service) authorizeAvatarChange(ctx context.Context, channelID, userNodeID string) (*store.Channel, error) {
	ch, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpManage)
	if err != nil {
		return nil, err
	}
	if ch.ChannelType == "dm" {
		return nil, fmt.Errorf("%w: a direct message has no group picture", ErrInvalidInput)
	}
	return ch, nil
}

// SetChannelAvatar makes b the group's picture and returns the URL the channel
// now points at. The picture is re-encoded, so nothing of the original file but
// its pixels is kept: no EXIF, no location, no other metadata. Each upload gets
// a new random token, so the URL says nothing about the picture.
func (s *Service) SetChannelAvatar(ctx context.Context, channelID, userID, userNodeID string, b []byte) (string, error) {
	if userID == "" {
		return "", ErrInvalidInput
	}
	ch, err := s.authorizeAvatarChange(ctx, channelID, userNodeID)
	if err != nil {
		return "", err
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
	if err := s.store.SetChannelAvatar(ctx, ch.ID, token, img, url); err != nil {
		if errors.Is(err, store.ErrNoSuchChannel) {
			return "", fmt.Errorf("%w: channel not found", ErrNotFound)
		}
		return "", fmt.Errorf("set channel avatar: %w", err)
	}
	s.announceChannelChange(ctx, ch)
	return url, nil
}

// ClearChannelAvatar removes the group's picture; its initials show again.
func (s *Service) ClearChannelAvatar(ctx context.Context, channelID, userNodeID string) error {
	ch, err := s.authorizeAvatarChange(ctx, channelID, userNodeID)
	if err != nil {
		return err
	}
	if err := s.store.ClearChannelAvatar(ctx, ch.ID); err != nil {
		if errors.Is(err, store.ErrNoSuchChannel) {
			return fmt.Errorf("%w: channel not found", ErrNotFound)
		}
		return fmt.Errorf("clear channel avatar: %w", err)
	}
	s.announceChannelChange(ctx, ch)
	return nil
}

// announceChannelChange tells the channel's own subscribers that what they
// show of it changed. It is the event a rename sends and carries only ids:
// clients fetch the channel again.
func (s *Service) announceChannelChange(ctx context.Context, ch *store.Channel) {
	ev := channelEvent(ctx, realtime.KindRenamed, ch)
	ev.ChannelID = ch.ID
	s.emit(ev)
}

// ChannelAvatar returns one size (64, 192 or 1024) of the picture under token,
// as a JPEG. Anything that names no stored picture is ErrNotFound.
func (s *Service) ChannelAvatar(ctx context.Context, token string, size int) ([]byte, error) {
	if !avatarToken.MatchString(token) {
		return nil, ErrNotFound
	}
	switch size {
	case avatarSmall, avatarMedium, avatarLarge:
	default:
		return nil, ErrNotFound
	}
	b, err := s.store.GetChannelAvatar(ctx, token, size)
	if err != nil {
		return nil, fmt.Errorf("get channel avatar: %w", err)
	}
	if b == nil {
		return nil, ErrNotFound
	}
	return b, nil
}

// TakePublicAvatarRequest counts one picture fetch from ip and returns
// ErrRateLimited (with the time left in the window) when the address has used
// up its allowance. The address is stored hashed: the counter needs to tell
// addresses apart, not to remember them.
func (s *Service) TakePublicAvatarRequest(ctx context.Context, ip string) error {
	if s.rdb == nil {
		return nil
	}
	sum := sha256.Sum256([]byte(ip))
	key := avatarPublicKey + hex.EncodeToString(sum[:8])
	return s.count(ctx, key, AvatarPublicLimit, AvatarPublicWindow)
}

// takeAvatarUpload spends one of the person's uploads for this hour. Without
// Redis (only unit tests run that way) nothing is counted.
func (s *Service) takeAvatarUpload(ctx context.Context, userID string) error {
	if s.rdb == nil {
		return nil
	}
	return s.count(ctx, avatarRateKey+userID, AvatarUploadLimit, avatarUploadWindow)
}

// count adds one to the window's counter at key and refuses past max.
func (s *Service) count(ctx context.Context, key string, max int64, window time.Duration) error {
	n, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("avatar rate limit: %w", err)
	}
	if n == 1 {
		s.rdb.Expire(ctx, key, window)
	}
	if n <= max {
		return nil
	}
	return &rateLimitedError{after: s.windowLeft(ctx, key, window)}
}

// windowLeft is how long a counter has to live. A counter that lost its expiry
// (the process died between INCR and EXPIRE) would lock the person out for
// good, so it is given one again.
func (s *Service) windowLeft(ctx context.Context, key string, window time.Duration) time.Duration {
	ttl, err := s.rdb.TTL(ctx, key).Result()
	if err != nil || ttl <= 0 {
		s.rdb.Expire(ctx, key, window)
		return window
	}
	return ttl
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
