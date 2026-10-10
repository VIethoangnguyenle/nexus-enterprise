package domain

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Proving the address of the account you are signed in to.
//
// Signing in by code or Google answers "who are you", and it issues a session.
// Here the person already has a session and the question is only "is this
// address yours": a code delivered to it, or a Google account whose own verified
// email is it. Neither issues a session, switches account or changes a tenant;
// the one effect is that the address becomes verified, so workspace invitations
// addressed to it appear.

const (
	emailVerifyKeyPrefix         = "emailverify:"
	emailVerifyAttemptsKeyPrefix = "emailverify_attempts:"
)

// emailVerifySession is what a requested code is stored as: the address it was
// sent to and an HMAC of the code, never the code. It is keyed by the account
// that asked, so it can be confirmed by that account only.
type emailVerifySession struct {
	Email    string `json:"email"`
	CodeHash string `json:"code_hash"`
}

func emailVerifyHash(key []byte, userID, email, code string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte("emailverify\x00" + userID + "\x00" + email + "\x00" + code))
	return hex.EncodeToString(m.Sum(nil))
}

// RequestEmailVerification sends a code to the address on the account, and
// returns how long the code lives. It is available only when a code delivered
// here reaches only the owner of the address (a real mail sender; not the fixed
// test code and not the log), because anything else would "prove" the address to
// whoever can read a log or knows the code.
func (s *Service) RequestEmailVerification(ctx context.Context, userID string) (time.Duration, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return 0, ErrNotFound
	}
	email := strings.ToLower(strings.TrimSpace(user.Email))
	if email == "" {
		return 0, ErrInvalidInput
	}
	if user.EmailVerified {
		return 0, ErrAlreadyVerified
	}
	if !s.OTPEnabled() || !s.OTPProvesOwnership() {
		return 0, ErrVerificationUnavailable
	}
	// The same allowance as asking for a sign-in code for this address: mailing
	// someone is mailing someone, whichever door asked.
	if err := s.checkOTPRate(ctx, "email", email); err != nil {
		return 0, err
	}

	code, err := generateOTPCode()
	if err != nil {
		return 0, err
	}
	data, err := json.Marshal(emailVerifySession{Email: email, CodeHash: emailVerifyHash(s.otp.Secret, userID, email, code)})
	if err != nil {
		return 0, fmt.Errorf("marshal email verification: %w", err)
	}
	key := emailVerifyKeyPrefix + userID
	if err := s.rdb.Set(ctx, key, data, otpTTL).Err(); err != nil {
		return 0, fmt.Errorf("store email verification: %w", err)
	}
	s.rdb.Del(ctx, emailVerifyAttemptsKeyPrefix+userID)
	if err := s.otp.Sender.SendCode(ctx, email, "email", code); err != nil {
		s.rdb.Del(ctx, key)
		return 0, fmt.Errorf("deliver email verification code: %w", err)
	}
	slog.Info("email verification code issued", "user_id", userID, "identifier", maskIdentifier(email, "email"))
	return otpTTL, nil
}

// ConfirmEmailVerification checks the code the account asked for and, if it is
// right, proves the account's address. keepSessionID is the caller's own
// session: the first proof ends every other session on the account (whoever
// opened them never held the mailbox) but not this one.
//
// At most 5 wrong codes are allowed per request; a code works once; and it works
// only for the account that asked and only while the address is still the one it
// was sent to. It returns no token: nothing is signed in.
func (s *Service) ConfirmEmailVerification(ctx context.Context, userID, keepSessionID, code string) error {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return ErrNotFound
	}
	if s.rdb == nil {
		return ErrVerificationUnavailable
	}

	key := emailVerifyKeyPrefix + userID
	data, err := s.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return ErrOTPExpired
	}
	if err != nil {
		return fmt.Errorf("read email verification: %w", err)
	}
	var sess emailVerifySession
	if err := json.Unmarshal(data, &sess); err != nil {
		return ErrOTPExpired
	}

	attemptsKey := emailVerifyAttemptsKeyPrefix + userID
	attempts, err := s.rdb.Incr(ctx, attemptsKey).Result()
	if err != nil {
		return fmt.Errorf("count email verification attempt: %w", err)
	}
	if attempts == 1 {
		s.rdb.Expire(ctx, attemptsKey, otpTTL)
	}
	if attempts > otpMaxAttempts {
		s.rdb.Del(ctx, key, attemptsKey)
		return ErrTooManyAttempts
	}

	current := strings.ToLower(strings.TrimSpace(user.Email))
	want, _ := hex.DecodeString(sess.CodeHash)
	got, _ := hex.DecodeString(emailVerifyHash(s.otp.Secret, userID, sess.Email, code))
	if sess.Email != current || len(code) != 6 || !hmac.Equal(got, want) {
		if sess.Email != current {
			// The address changed after the code was sent: it was for another one.
			s.rdb.Del(ctx, key, attemptsKey)
			return ErrOTPExpired
		}
		return &otpInvalidError{left: otpMaxAttempts - int(attempts)}
	}

	if n, err := s.rdb.Del(ctx, key).Result(); err != nil || n == 0 {
		return ErrOTPExpired
	}
	s.rdb.Del(ctx, attemptsKey)
	return s.proveAddress(ctx, userID, keepSessionID, "code")
}

// VerifyEmailWithGoogle proves the account's address through a Google account,
// and only when that Google account's own verified email is the address on the
// account. A different address, or one Google has not verified, is refused and
// nothing changes: in particular this never signs in as the Google account, never
// links it and never creates anything, because the person chose to verify the
// account they are in, not to become someone else.
func (s *Service) VerifyEmailWithGoogle(ctx context.Context, userID, keepSessionID string, ext ExternalIdentity) error {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return ErrNotFound
	}
	if !ext.EmailVerified {
		return ErrEmailNotVerified
	}
	theirs := strings.ToLower(strings.TrimSpace(ext.Email))
	mine := strings.ToLower(strings.TrimSpace(user.Email))
	if theirs == "" || mine == "" || theirs != mine {
		return ErrEmailMismatch
	}
	return s.proveAddress(ctx, userID, keepSessionID, "google")
}

// proveAddress records the proof. The first proof on an account ends whatever
// was set up before it without proof (a password, other sessions), exactly as
// Google linking and a delivered sign-in code do, but spares the caller's own
// session.
func (s *Service) proveAddress(ctx context.Context, userID, keepSessionID, how string) error {
	changed, err := s.store.MarkEmailVerified(ctx, userID)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if err := s.evictUnverifiedCredentialsExcept(ctx, userID, keepSessionID); err != nil {
		return err
	}
	slog.Info("audit: address proved for the signed-in account", "user_id", userID, "by", how)
	return nil
}
