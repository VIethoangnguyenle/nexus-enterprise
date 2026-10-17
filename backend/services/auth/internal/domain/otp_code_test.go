package domain

import (
	"regexp"
	"testing"
)

var sixDigits = regexp.MustCompile(`^[0-9]{6}$`)

func TestGenerateOTPCode_IsSixDigitsAndRandom(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		code, err := generateOTPCode()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if !sixDigits.MatchString(code) {
			t.Fatalf("code %q is not exactly six digits", code)
		}
		seen[code] = true
	}
	// 200 draws from 10^6 values: a handful of collisions at most.
	if len(seen) < 190 {
		t.Errorf("only %d distinct codes in 200 draws — not random", len(seen))
	}
}

func TestOTPCodeHash_BindsCodeToSessionAndKey(t *testing.T) {
	key := []byte("server-secret")
	h := hashOTPCode(key, "session-a", "123456")

	if h == "123456" || h == "" {
		t.Fatal("the stored value must not be the code itself")
	}
	if !otpCodeMatches(key, "session-a", "123456", h) {
		t.Error("the right code must match")
	}
	for _, tc := range []struct {
		name               string
		key                []byte
		session, candidate string
	}{
		{"wrong code", key, "session-a", "123457"},
		{"other session", key, "session-b", "123456"},
		{"other key", []byte("different"), "session-a", "123456"},
		{"empty", key, "session-a", ""},
	} {
		if otpCodeMatches(tc.key, tc.session, tc.candidate, h) {
			t.Errorf("%s: must not match", tc.name)
		}
	}
	if otpCodeMatches(key, "session-a", "123456", "") {
		t.Error("a session without a stored hash must never match")
	}
}

func TestIsTestAccount(t *testing.T) {
	for _, tc := range []struct {
		name, ident, identType string
		want                   bool
	}{
		{"example.test", "x@example.test", "email", true},
		{"subdomain", "x@a.b.foo.test", "email", true},
		{"bare tld host", "x@test", "email", true},
		{"uppercase", "X@Example.TEST", "email", true},
		{"test.com", "x@test.com", "email", false},
		{"testing tld", "x@example.testing", "email", false},
		{"suffix without dot", "x@footest", "email", false},
		{"test in local part", "x.test@example.com", "email", false},
		{"phone", "+84901234567", "phone", false},
		{"phone type with .test text", "x@example.test", "phone", false},
		{"no at sign", "example.test", "email", false},
		{"empty", "", "email", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTestAccount(tc.ident, tc.identType); got != tc.want {
				t.Errorf("isTestAccount(%q, %q) = %v, want %v", tc.ident, tc.identType, got, tc.want)
			}
		})
	}
}
