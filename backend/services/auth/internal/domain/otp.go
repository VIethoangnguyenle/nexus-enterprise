package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/store"
)

const (
	otpKeyPrefix   = "otp:"
	otpTTL         = 5 * time.Minute
	otpMaxAttempts = 5
	otpHardcoded   = "999999"
)

// otpSession is the Redis-stored OTP session data.
type otpSession struct {
	Identifier string `json:"identifier"`
	Type       string `json:"type"`
	Code       string `json:"code"`
	Attempts   int    `json:"attempts"`
}

// OTPResult is the domain output for OTP verification.
type OTPResult struct {
	Token      string
	UserID     string
	Username   string
	NGACNodeID string
	Email      string
	Phone      string
	UnionID    string
	IsNewUser  bool
}

// RequestOTP validates the identifier and creates an OTP session in Redis.
func (s *Service) RequestOTP(ctx context.Context, identifier, identType string) (string, error) {
	if s.rdb == nil {
		return "", fmt.Errorf("redis unavailable for OTP")
	}

	normalized, err := validateIdentifier(identifier, identType)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}

	sessionID := uuid.New().String()
	session := otpSession{
		Identifier: normalized,
		Type:       identType,
		Code:       otpHardcoded,
		Attempts:   0,
	}

	data, err := json.Marshal(session)
	if err != nil {
		return "", fmt.Errorf("marshal otp session: %w", err)
	}

	if err := s.rdb.Set(ctx, otpKeyPrefix+sessionID, data, otpTTL).Err(); err != nil {
		return "", fmt.Errorf("store otp session: %w", err)
	}

	slog.Info("OTP generated", "session_id", sessionID, "identifier", maskIdentifier(normalized, identType), "code", otpHardcoded)
	return sessionID, nil
}

// VerifyOTP checks the code against the Redis session.
// If the identifier is new, auto-registers the user and provisions workspace.
func (s *Service) VerifyOTP(ctx context.Context, sessionID, code string) (*OTPResult, error) {
	if s.rdb == nil {
		return nil, fmt.Errorf("redis unavailable for OTP")
	}

	key := otpKeyPrefix + sessionID
	data, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, ErrOTPExpired
	}

	var session otpSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal otp session: %w", err)
	}

	if session.Attempts >= otpMaxAttempts {
		s.rdb.Del(ctx, key)
		return nil, ErrTooManyAttempts
	}

	if session.Code != code {
		session.Attempts++
		updated, _ := json.Marshal(session)
		s.rdb.Set(ctx, key, updated, s.rdb.TTL(ctx, key).Val())
		return nil, ErrOTPInvalid
	}

	// OTP matched — delete session (one-time use)
	s.rdb.Del(ctx, key)

	return s.resolveOTPUser(ctx, session.Identifier, session.Type)
}

// resolveOTPUser finds an existing user or creates a new one.
func (s *Service) resolveOTPUser(ctx context.Context, identifier, identType string) (*OTPResult, error) {
	var user *OTPResult
	var err error

	if identType == "phone" {
		user, err = s.findUserByPhone(ctx, identifier)
	} else {
		user, err = s.findUserByEmail(ctx, identifier)
	}
	if err != nil {
		return nil, err
	}

	if user != nil {
		return user, nil
	}

	// New user — auto-register
	return s.createOTPUser(ctx, identifier, identType)
}

// findUserByPhone looks up an existing user by phone and generates a JWT.
func (s *Service) findUserByPhone(ctx context.Context, phone string) (*OTPResult, error) {
	user, err := s.store.GetUserByPhone(ctx, phone)
	if err != nil {
		return nil, fmt.Errorf("get user by phone: %w", err)
	}
	if user == nil {
		return nil, nil
	}
	return s.otpResultFromExistingUser(ctx, user)
}

// findUserByEmail looks up an existing user by email and generates a JWT.
func (s *Service) findUserByEmail(ctx context.Context, email string) (*OTPResult, error) {
	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	if user == nil {
		return nil, nil
	}
	return s.otpResultFromExistingUser(ctx, user)
}

// otpResultFromExistingUser generates a JWT and builds OTPResult for a known user.
func (s *Service) otpResultFromExistingUser(ctx context.Context, u *store.User) (*OTPResult, error) {
	tenants, _ := s.store.ListTenantsByUser(ctx, u.ID)
	defaultTenantID := s.selectDefaultTenant(tenants)

	token, err := auth.GenerateToken(u.ID, u.Username, u.NGACNodeID, defaultTenantID)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &OTPResult{
		Token:      token,
		UserID:     u.ID,
		Username:   u.Username,
		NGACNodeID: u.NGACNodeID,
		Email:      u.Email,
		Phone:      u.Phone,
		UnionID:    u.UnionID,
		IsNewUser:  false,
	}, nil
}

// createOTPUser creates a new user from an OTP-verified identifier.
func (s *Service) createOTPUser(ctx context.Context, identifier, identType string) (*OTPResult, error) {
	var username, email, phone string

	if identType == "phone" {
		phone = identifier
		username = "user_" + strings.ReplaceAll(identifier, "+", "")
		email = ""
	} else {
		email = identifier
		username = emailToUsername(email)
		phone = ""
	}

	ngacNode, err := s.createUserNGACNode(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("create ngac node: %w", err)
	}

	userID := uuid.New().String()
	unionID := uuid.New().String()
	displayName := username

	if err := s.store.CreateUser(ctx, userID, username, "", ngacNode, email, unionID, displayName, phone); err != nil {
		return nil, fmt.Errorf("create otp user: %w", err)
	}

	// Auto-provision workspace + #general channel
	s.autoProvisionWorkspace(ctx, userID, username, ngacNode)

	tenants, _ := s.store.ListTenantsByUser(ctx, userID)
	defaultTenantID := s.selectDefaultTenant(tenants)

	token, err := auth.GenerateToken(userID, username, ngacNode, defaultTenantID)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	slog.Info("OTP auto-registered new user", "user_id", userID, "username", username, "type", identType)

	return &OTPResult{
		Token:      token,
		UserID:     userID,
		Username:   username,
		NGACNodeID: ngacNode,
		Email:      email,
		Phone:      phone,
		UnionID:    unionID,
		IsNewUser:  true,
	}, nil
}

// --- Validation helpers ---

var (
	phoneRegex = regexp.MustCompile(`^(0[3-9][0-9]{8,9})$`)
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

// validateIdentifier validates and normalizes a phone or email identifier.
func validateIdentifier(identifier, identType string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", fmt.Errorf("identifier is required")
	}

	if identType == "phone" {
		return normalizePhone(identifier)
	}
	if identType == "email" {
		return normalizeEmail(identifier)
	}
	return "", fmt.Errorf("type must be 'phone' or 'email'")
}

// normalizePhone normalizes Vietnamese phone numbers to 0XXXXXXXXX format.
// Accepts: 0912345678, +84912345678, 84912345678
func normalizePhone(phone string) (string, error) {
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")

	if strings.HasPrefix(phone, "+84") {
		phone = "0" + phone[3:]
	} else if strings.HasPrefix(phone, "84") && len(phone) > 9 {
		phone = "0" + phone[2:]
	}

	if !phoneRegex.MatchString(phone) {
		return "", fmt.Errorf("invalid phone number format")
	}
	return phone, nil
}

// normalizeEmail validates and lowercases an email address.
func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !emailRegex.MatchString(email) {
		return "", fmt.Errorf("invalid email format")
	}
	return email, nil
}

// maskIdentifier masks an identifier for safe logging.
func maskIdentifier(identifier, identType string) string {
	if identType == "phone" && len(identifier) >= 7 {
		return identifier[:3] + "****" + identifier[len(identifier)-3:]
	}
	if identType == "email" {
		parts := strings.SplitN(identifier, "@", 2)
		if len(parts) == 2 && len(parts[0]) > 2 {
			return parts[0][:2] + "***@" + parts[1]
		}
	}
	return "***"
}
