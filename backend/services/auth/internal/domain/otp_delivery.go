package domain

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
)

// DefaultFixedOTPCode is the test-mode code used when AUTH_FIXED_OTP_CODE is
// not set at all. See OTPOptions.FixedCode.
const DefaultFixedOTPCode = "999999"

// CodeSender delivers a one-time code to the identifier it was issued for.
// Delivery is what makes an OTP proof of anything: a code the user could not
// have received proves nothing.
type CodeSender interface {
	SendCode(ctx context.Context, identifier, identType, code string) error
}

// OwnershipProver is implemented by a sender whose codes reach only the owner of
// the address or number: a real mail or SMS provider. Delivery through the log,
// or a fixed code, reaches whoever can read the log or knows the code, so those
// prove nothing and a code accepted through them does not verify an address.
type OwnershipProver interface {
	DeliversToOwner() bool
}

// OTPOptions configures one-time-code sign-in.
type OTPOptions struct {
	// FixedCode, when non-empty, is the code every OTP session accepts — a
	// documented TEST-ONLY mode (AUTH_FIXED_OTP_CODE, default "999999") so
	// testers can sign in to any account without a mailbox. It proves nothing
	// about the identifier. Empty switches to random, delivered codes.
	FixedCode string
	// Sender delivers random codes. Without one, and without FixedCode, OTP
	// sign-in is disabled.
	Sender CodeSender
	// Secret keys the HMAC under which codes are stored. Instances serving
	// the same Redis must share it. Empty keeps a per-process random key.
	Secret []byte
}

// ConfigureOTP replaces the OTP configuration. Call before serving traffic.
func (s *Service) ConfigureOTP(o OTPOptions) {
	if len(o.Secret) == 0 {
		o.Secret = s.otp.Secret
	}
	s.otp = o
}

// OTPEnabled reports whether RequestOTP can issue codes at all.
func (s *Service) OTPEnabled() bool {
	return s != nil && s.rdb != nil && (s.otp.FixedCode != "" || s.otp.Sender != nil)
}

// OTPProvesOwnership reports whether a code accepted now proves the holder
// controls the address: random codes (not the fixed test code) delivered by a
// sender that says it delivers to the owner. LogSender does not.
func (s *Service) OTPProvesOwnership() bool {
	if s == nil || s.otp.FixedCode != "" || s.otp.Sender == nil {
		return false
	}
	p, ok := s.otp.Sender.(OwnershipProver)
	return ok && p.DeliversToOwner()
}

// OTPFixedCodeActive reports whether the fixed test code is in force.
func (s *Service) OTPFixedCodeActive() bool {
	return s.OTPEnabled() && s.otp.FixedCode != ""
}

// DevOTPMode reports whether codes may be delivered through the log:
// APP_ENV=dev or AUTH_DEV_OTP=1.
func DevOTPMode() bool {
	return os.Getenv("APP_ENV") == "dev" || os.Getenv("AUTH_DEV_OTP") == "1"
}

// LogSender "delivers" codes by logging them, for local development only. It
// re-checks DevOTPMode on every call and refuses outside it, so wiring it into
// a non-dev deployment by mistake cannot leak codes into production logs.
type LogSender struct{}

// SendCode logs the code in dev mode and refuses otherwise.
func (LogSender) SendCode(_ context.Context, identifier, identType, code string) error {
	if !DevOTPMode() {
		return errors.New("log delivery of OTP codes is only allowed in dev mode")
	}
	slog.Info("DEV ONLY: OTP code", "identifier", maskIdentifier(identifier, identType), "code", code)
	return nil
}

// newOTPSecret returns a random HMAC key for when none is configured.
func newOTPSecret() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand unavailable: %v", err))
	}
	return b
}

var otpCodeSpace = big.NewInt(1_000_000)

// generateOTPCode returns a uniformly random six-digit code. rand.Int draws
// from [0, 10^6) by rejection sampling, so there is no modulo bias.
func generateOTPCode() (string, error) {
	n, err := rand.Int(rand.Reader, otpCodeSpace)
	if err != nil {
		return "", fmt.Errorf("generate otp code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// hashOTPCode is the at-rest form of a code: HMAC-SHA256 keyed by the server
// secret, bound to the session so a hash cannot be replayed onto another one.
func hashOTPCode(key []byte, sessionID, code string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(sessionID))
	m.Write([]byte{0})
	m.Write([]byte(code))
	return hex.EncodeToString(m.Sum(nil))
}

// otpCodeMatches compares a candidate against the stored hash in constant time.
func otpCodeMatches(key []byte, sessionID, candidate, storedHash string) bool {
	if storedHash == "" || candidate == "" {
		return false
	}
	want, err := hex.DecodeString(storedHash)
	if err != nil {
		return false
	}
	got, _ := hex.DecodeString(hashOTPCode(key, sessionID, candidate))
	return hmac.Equal(got, want)
}
