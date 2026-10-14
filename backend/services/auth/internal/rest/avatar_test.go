package rest

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/store"
)

// --- the avatar half of the fake store ---

type acctAvatar struct {
	userID string
	img    store.AvatarImages
}

func (s *acctStore) SetAvatar(_ context.Context, userID, token string, img store.AvatarImages, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[userID]
	if u == nil {
		return store.ErrNoSuchUser
	}
	if s.avatars == nil {
		s.avatars = map[string]acctAvatar{}
	}
	for tok, a := range s.avatars {
		if a.userID == userID {
			delete(s.avatars, tok)
		}
	}
	s.avatars[token] = acctAvatar{userID: userID, img: img}
	u.AvatarURL = url
	return nil
}

func (s *acctStore) ClearAvatar(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[userID]
	if u == nil {
		return store.ErrNoSuchUser
	}
	for tok, a := range s.avatars {
		if a.userID == userID {
			delete(s.avatars, tok)
		}
	}
	u.AvatarURL = ""
	return nil
}

func (s *acctStore) GetAvatar(_ context.Context, token string, size int) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.avatars[token]
	if !ok {
		return nil, nil
	}
	return map[int][]byte{64: a.img.Small, 192: a.img.Medium, 1024: a.img.Large}[size], nil
}

// --- requests ---

func smallPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	img.Set(0, 0, color.Black)
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// multipartBody is a form with one file part named field.
func multipartBody(t *testing.T, field, filename string, content []byte) (io.Reader, string) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &b, w.FormDataContentType()
}

func (a *acctApp) upload(token string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, "/api/me/avatar", body)
	if contentType != "" {
		req.Header.Set(echo.HeaderContentType, contentType)
	}
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.e.ServeHTTP(rec, req)
	return rec
}

func (a *acctApp) uploadFile(t *testing.T, userID string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := multipartBody(t, "file", "me.png", content)
	return a.upload(a.token(userID), body, ct)
}

func wantCode(t *testing.T, name string, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Errorf("%s: status %d (%s), want %d %s", name, rec.Code, rec.Body, status, code)
		return
	}
	if got := decode(t, rec)["code"]; got != code {
		t.Errorf("%s: code %v, want %s", name, got, code)
	}
}

// --- upload ---

func TestUploadAvatar_StoresItAndAnswersTheURL(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")

	rec := a.uploadFile(t, "u1", smallPNG(t))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	url, _ := decode(t, rec)["avatar_url"].(string)
	if !strings.HasPrefix(url, domain.AvatarURLPrefix) || len(url) != len(domain.AvatarURLPrefix)+32 {
		t.Fatalf("avatar_url = %q", url)
	}
	if a.st.users["u1"].AvatarURL != url {
		t.Errorf("profile avatar = %q, want %q", a.st.users["u1"].AvatarURL, url)
	}
}

func TestUploadAvatar_Refusals(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")
	tok := a.token("u1")

	t.Run("no session", func(t *testing.T) {
		body, ct := multipartBody(t, "file", "me.png", smallPNG(t))
		if rec := a.upload("", body, ct); rec.Code != http.StatusUnauthorized {
			t.Errorf("status %d, want 401", rec.Code)
		}
	})
	t.Run("file over 5 MiB", func(t *testing.T) {
		big := append(smallPNG(t), make([]byte, domain.AvatarMaxBytes)...)
		body, ct := multipartBody(t, "file", "me.png", big)
		wantCode(t, "over", a.upload(tok, body, ct), http.StatusRequestEntityTooLarge, "avatar_too_large")
	})
	t.Run("file of exactly 5 MiB + 1", func(t *testing.T) {
		body, ct := multipartBody(t, "file", "me.png", make([]byte, domain.AvatarMaxBytes+1))
		wantCode(t, "5MiB+1", a.upload(tok, body, ct), http.StatusRequestEntityTooLarge, "avatar_too_large")
	})
	t.Run("a body far over the limit is cut off, not read", func(t *testing.T) {
		body, ct := multipartBody(t, "file", "me.png", make([]byte, 3*domain.AvatarMaxBytes))
		wantCode(t, "huge", a.upload(tok, body, ct), http.StatusRequestEntityTooLarge, "avatar_too_large")
	})
	t.Run("text named .png", func(t *testing.T) {
		wantCode(t, "text", a.uploadFile(t, "u1", []byte("<svg onload=alert(1)>")), http.StatusBadRequest, "avatar_not_image")
	})
	t.Run("too many pixels", func(t *testing.T) {
		header := pngHeader(10000, 10000)
		wantCode(t, "pixels", a.uploadFile(t, "u1", header), http.StatusBadRequest, "avatar_too_many_pixels")
	})
	t.Run("no file part", func(t *testing.T) {
		body, ct := multipartBody(t, "picture", "me.png", smallPNG(t))
		wantCode(t, "field", a.upload(tok, body, ct), http.StatusBadRequest, "avatar_missing")
	})
	t.Run("JSON instead of a form", func(t *testing.T) {
		wantCode(t, "json", a.upload(tok, strings.NewReader(`{"avatar_url":"x"}`), echo.MIMEApplicationJSON), http.StatusBadRequest, "invalid_input")
	})
	t.Run("upload cut off half-way", func(t *testing.T) {
		body, ct := multipartBody(t, "file", "me.png", smallPNG(t))
		all, _ := io.ReadAll(body)
		wantCode(t, "cut", a.upload(tok, bytes.NewReader(all[:len(all)/2]), ct), http.StatusBadRequest, "invalid_input")
	})
	t.Run("account deleted while its token is still valid", func(t *testing.T) {
		a.st.add("gone", "")
		gone := a.token("gone")
		delete(a.st.users, "gone")
		body, ct := multipartBody(t, "file", "me.png", smallPNG(t))
		wantCode(t, "gone", a.upload(gone, body, ct), http.StatusNotFound, "not_found")
	})
	if a.st.users["u1"].AvatarURL != "" {
		t.Errorf("a refused upload changed the profile: %q", a.st.users["u1"].AvatarURL)
	}
}

// A busy answer comes from the domain only while another picture is being
// decoded, which a request cannot arrange; its HTTP form is checked directly.
func TestUploadAvatar_BusyAsksToComeBack(t *testing.T) {
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(httptest.NewRequest(http.MethodPut, "/api/me/avatar", nil), rec)
	he, ok := failAvatar(c, domain.ErrAvatarBusy).(*echo.HTTPError)
	if !ok || he.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %v, want a 503", he)
	}
	if body, _ := he.Message.(map[string]any); body["code"] != "avatar_busy" {
		t.Errorf("body %v, want code avatar_busy", he.Message)
	}
	if rec.Header().Get(echo.HeaderRetryAfter) == "" {
		t.Error("a busy answer says when to come back")
	}
}

func TestUploadAvatar_RateLimited(t *testing.T) {
	a := newAcctApp(t, true, nil)
	id := uniq("rl")
	a.st.add(id, "")
	for i := 0; i < domain.AvatarUploadLimit; i++ {
		if rec := a.uploadFile(t, id, smallPNG(t)); rec.Code != http.StatusOK {
			t.Fatalf("upload %d: %d %s", i+1, rec.Code, rec.Body)
		}
	}
	rec := a.uploadFile(t, id, smallPNG(t))
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get(echo.HeaderRetryAfter) == "" {
		t.Errorf("upload %d: %d (Retry-After %q), want 429 with Retry-After", domain.AvatarUploadLimit+1, rec.Code, rec.Header().Get(echo.HeaderRetryAfter))
	}
}

// --- remove ---

func TestRemoveAvatar(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")
	if rec := a.uploadFile(t, "u1", smallPNG(t)); rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}

	if rec := a.do(http.MethodDelete, "/api/me/avatar", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no session: %d, want 401", rec.Code)
	}
	rec := a.do(http.MethodDelete, "/api/me/avatar", a.token("u1"), "")
	if rec.Code != http.StatusOK || decode(t, rec)["avatar_url"] != "" {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if a.st.users["u1"].AvatarURL != "" {
		t.Errorf("profile avatar = %q after removing it", a.st.users["u1"].AvatarURL)
	}
}

// --- serve ---

func TestServeAvatar_EverySizeWithItsHeaders(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")
	rec := a.uploadFile(t, "u1", smallPNG(t))
	url, _ := decode(t, rec)["avatar_url"].(string)

	for path, size := range map[string]int{url: 192, url + "/64": 64, url + "/192": 192, url + "/1024": 1024} {
		rec := a.do(http.MethodGet, path, "", "")
		if rec.Code != http.StatusOK {
			t.Errorf("%s: %d %s", path, rec.Code, rec.Body)
			continue
		}
		img, format, err := image.Decode(bytes.NewReader(rec.Body.Bytes()))
		if err != nil || format != "jpeg" || img.Bounds().Dx() != size {
			t.Errorf("%s: %v %q %v, want a %d px JPEG", path, err, format, img, size)
		}
		h := rec.Header()
		for k, v := range map[string]string{
			"Content-Type":                 "image/jpeg",
			"Cache-Control":                "public, max-age=31536000, immutable",
			"X-Content-Type-Options":       "nosniff",
			"Content-Security-Policy":      "default-src 'none'",
			"Cross-Origin-Resource-Policy": "same-origin",
			"Content-Disposition":          `inline; filename="avatar.jpg"`,
		} {
			if h.Get(k) != v {
				t.Errorf("%s: %s = %q, want %q", path, k, h.Get(k), v)
			}
		}
	}
}

func TestServeAvatar_AnythingElseIsNotFound(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")
	rec := a.uploadFile(t, "u1", smallPNG(t))
	url, _ := decode(t, rec)["avatar_url"].(string)

	for _, path := range []string{
		domain.AvatarURLPrefix + strings.Repeat("0", 32), // nobody's
		url + "/100", // a size that is not kept
		url + "/big", // not a size at all
		domain.AvatarURLPrefix + "short",
	} {
		rec := a.do(http.MethodGet, path, "", "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, rec.Code)
			continue
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=60" {
			t.Errorf("%s: Cache-Control %q, want a short public cache", path, cc)
		}
	}
}

func TestServeAvatar_IsLimitedPerAddress(t *testing.T) {
	a := limitedApp(t, 1, nil) // avatars get ten times the sign-in allowance
	a.st.add("u1", "a@example.test")
	path := domain.AvatarURLPrefix + strings.Repeat("0", 32) + "/64"
	remote := uniqueRemote()
	var last int
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		a.e.ServeHTTP(rec, req)
		last = rec.Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("request 11 within the minute: %d, want 429", last)
	}
}

// pngHeader is the signature and IHDR of an 8-bit RGBA PNG of the given size,
// with no pixels: what a decompression bomb looks like to DecodeConfig.
func pngHeader(w, h int) []byte {
	var ihdr bytes.Buffer
	ihdr.WriteString("IHDR")
	binary.Write(&ihdr, binary.BigEndian, uint32(w))
	binary.Write(&ihdr, binary.BigEndian, uint32(h))
	ihdr.Write([]byte{8, 6, 0, 0, 0})
	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	binary.Write(&out, binary.BigEndian, uint32(ihdr.Len()-4))
	out.Write(ihdr.Bytes())
	binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(ihdr.Bytes()))
	return out.Bytes()
}
