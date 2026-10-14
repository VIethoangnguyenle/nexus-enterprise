package domain

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png" // registers the PNG decoder with image.Decode
	"regexp"
	"time"

	xdraw "golang.org/x/image/draw"

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

	// avatarDecodeBudget is how much memory decoding one picture may take. The
	// auth service has 192 MB in production; this leaves room for everything
	// else it does while a picture is being decoded. It admits a 12 MP phone
	// photo and refuses the small file that claims to be 10000×10000.
	avatarDecodeBudget = 48 << 20
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
	if err := checkDecodeBudget(b); err != nil {
		return store.AvatarImages{}, err
	}
	src, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return store.AvatarImages{}, ErrAvatarNotImage
	}

	// The centre square is the same square whichever way the picture is
	// meant to be turned, so the turn is applied to the small result instead
	// of to a full-size copy.
	r := src.Bounds()
	side := min(r.Dx(), r.Dy())
	x0, y0 := r.Min.X+(r.Dx()-side)/2, r.Min.Y+(r.Dy()-side)/2
	square := image.Rectangle{Min: image.Pt(x0, y0), Max: image.Pt(x0+side, y0+side)}

	large := orient(onWhite(halveTo(src, square, avatarLarge), avatarLarge), jpegOrientation(b))
	medium := halveTo(large, large.Bounds(), avatarMedium)
	small := halveTo(medium, medium.Bounds(), avatarSmall)

	var out store.AvatarImages
	for _, p := range []struct {
		img image.Image
		dst *[]byte
	}{{large, &out.Large}, {medium, &out.Medium}, {small, &out.Small}} {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, p.img, &jpeg.Options{Quality: avatarQuality}); err != nil {
			return store.AvatarImages{}, fmt.Errorf("encode avatar: %w", err)
		}
		*p.dst = buf.Bytes()
	}
	return out, nil
}

// halveTo scales the part r of src to a size×size picture. A large picture is
// halved step by step first: each halving with ApproxBiLinear is a 2×2 average,
// so quality holds, and unlike the kernel scalers (CatmullRom and friends,
// which keep a float64 buffer of width × source height) it needs no scratch
// memory beyond the result.
func halveTo(src image.Image, r image.Rectangle, size int) *image.RGBA {
	for r.Dx() > 2*size {
		half := image.NewRGBA(image.Rect(0, 0, r.Dx()/2, r.Dy()/2))
		xdraw.ApproxBiLinear.Scale(half, half.Bounds(), src, r, draw.Src, nil)
		src, r = half, half.Bounds()
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, r, draw.Src, nil)
	return dst
}

// onWhite lays img over white, so a transparent picture does not turn black
// once it is a JPEG.
func onWhite(img *image.RGBA, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Over)
	return dst
}

// orient turns a square picture the way its EXIF Orientation (1–8) says it is
// meant to be seen.
func orient(img *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return img
	}
	n := img.Bounds().Dx()
	dst := image.NewRGBA(img.Bounds())
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			var sx, sy int
			switch o {
			case 2: // mirrored
				sx, sy = n-1-x, y
			case 3: // half a turn
				sx, sy = n-1-x, n-1-y
			case 4: // mirrored upside down
				sx, sy = x, n-1-y
			case 5: // mirrored, a quarter turn counter-clockwise
				sx, sy = y, x
			case 6: // a quarter turn clockwise
				sx, sy = y, n-1-x
			case 7: // mirrored, a quarter turn clockwise
				sx, sy = n-1-y, n-1-x
			case 8: // a quarter turn counter-clockwise
				sx, sy = n-1-y, x
			}
			dst.SetRGBA(x, y, img.RGBAAt(sx, sy))
		}
	}
	return dst
}

// checkDecodeBudget refuses, from the header alone, a picture whose decode
// would take more than avatarDecodeBudget. A file that is not a JPEG or PNG
// is ErrAvatarNotImage.
func checkDecodeBudget(b []byte) error {
	n, err := decodedBytes(b)
	if err != nil {
		return err
	}
	if n > avatarDecodeBudget {
		return ErrAvatarTooManyPixels
	}
	return nil
}

// decodedBytes estimates the memory a decode of b takes: the decoded pixels,
// a progressive JPEG's coefficients, and the first quarter-size working copy
// of the centre square.
func decodedBytes(b []byte) (int64, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || (format != "jpeg" && format != "png") || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, ErrAvatarNotImage
	}
	w, h := int64(cfg.Width), int64(cfg.Height)
	side := min(w, h)
	working := side * side // (side/2)² RGBA

	if format == "jpeg" {
		samples, progressive, ok := jpegSamples(b)
		if !ok {
			// A frame the decoder reads but this parser cannot: assume the
			// worst, full-size planes decoded progressively.
			samples, progressive = w*h*3, true
		}
		decoded := samples
		if progressive {
			decoded += samples * 4 // one int32 coefficient per sample
		}
		return decoded + working, nil
	}

	var bpp int64
	switch cfg.ColorModel {
	case color.GrayModel:
		bpp = 1
	case color.Gray16Model:
		bpp = 2
	case color.RGBA64Model, color.NRGBA64Model:
		bpp = 8
	default:
		if _, paletted := cfg.ColorModel.(color.Palette); paletted {
			bpp = 1
		} else {
			bpp = 4
		}
	}
	return w*h*bpp + working, nil
}

// jpegSamples reads a JPEG's frame header: how many samples its components
// hold in all (Cb and Cr are usually a quarter of Y) and whether it is
// progressive.
func jpegSamples(b []byte) (samples int64, progressive, ok bool) {
	for i := 2; i+1 < len(b); {
		if b[i] != 0xFF {
			return 0, false, false
		}
		// Any number of 0xFF fill bytes may come before a marker. Losing
		// count of them here would read the wrong frame, or none.
		for i+1 < len(b) && b[i+1] == 0xFF {
			i++
		}
		if i+4 > len(b) {
			return 0, false, false
		}
		marker := b[i+1]
		length := int(binary.BigEndian.Uint16(b[i+2:]))
		seg := i + 4
		if length < 2 || seg+length-2 > len(b) {
			return 0, false, false
		}
		switch marker {
		case 0xC0, 0xC1, 0xC2:
			if length < 8 {
				return 0, false, false
			}
			h := int64(binary.BigEndian.Uint16(b[seg+1:]))
			w := int64(binary.BigEndian.Uint16(b[seg+3:]))
			n := int(b[seg+5])
			if n == 0 || length < 8+3*n {
				return 0, false, false
			}
			var maxH, maxV int64 = 1, 1
			for c := 0; c < n; c++ {
				hv := b[seg+6+3*c+1]
				if hv>>4 == 0 || hv&0x0F == 0 {
					return 0, false, false
				}
				maxH, maxV = max(maxH, int64(hv>>4)), max(maxV, int64(hv&0x0F))
			}
			for c := 0; c < n; c++ {
				hv := b[seg+6+3*c+1]
				samples += w * h * int64(hv>>4) * int64(hv&0x0F) / (maxH * maxV)
			}
			return samples, marker == 0xC2, true
		case 0xDA: // start of scan: no frame header before it
			return 0, false, false
		}
		i = seg + length - 2
	}
	return 0, false, false
}

// jpegOrientation reads the EXIF Orientation tag of a JPEG. Anything missing
// or malformed reads as 1, upright.
func jpegOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	for i := 2; i+1 < len(b); {
		if b[i] != 0xFF {
			return 1
		}
		for i+1 < len(b) && b[i+1] == 0xFF { // fill bytes before a marker
			i++
		}
		if i+4 > len(b) {
			return 1
		}
		marker := b[i+1]
		length := int(binary.BigEndian.Uint16(b[i+2:]))
		seg, end := i+4, i+2+length
		if length < 2 || end > len(b) || marker == 0xDA {
			return 1
		}
		if marker == 0xE1 && end-seg >= 14 && string(b[seg:seg+6]) == "Exif\x00\x00" {
			return exifOrientation(b[seg+6 : end])
		}
		i = end
	}
	return 1
}

// exifOrientation finds tag 0x0112 in the first IFD of a TIFF block.
func exifOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	ifd := int(order.Uint32(t[4:]))
	if ifd < 8 || ifd+2 > len(t) {
		return 1
	}
	count := int(order.Uint16(t[ifd:]))
	for e := 0; e < count; e++ {
		off := ifd + 2 + 12*e
		if off+12 > len(t) {
			return 1
		}
		if order.Uint16(t[off:]) == 0x0112 {
			if o := int(order.Uint16(t[off+8:])); o >= 1 && o <= 8 {
				return o
			}
			return 1
		}
	}
	return 1
}
