package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"ngac-platform/services/auth/internal/auth"
)

// Sign-in by QR code: a computer that is not signed in shows a code; a phone that
// is signed in scans it, sees which computer is asking, and approves. The computer
// then gets a session for the phone's person in the tenant the phone is using.
//
// The state of one code lives in Redis for QRLoginTTL and nowhere else. Two
// secrets are minted per code: the token, which the QR carries and anyone who
// sees the screen can read, and the poll secret, which stays in an httpOnly
// cookie of the browser that asked. Only a poll that presents the poll secret
// can collect the session, so a photo of the QR is worth nothing.

// QRLoginTTL is how long a code can be scanned and approved.
const QRLoginTTL = 2 * time.Minute

const qrLoginKeyPrefix = "qr_login:"

// ErrQRSessionEnded means the phone's session was ended (signed out, revoked)
// although its access token has not expired yet. Such a session must not hand
// a new one to a computer.
var ErrQRSessionEnded = errors.New("session has ended")

// ErrQRUnavailable means the code cannot be used: never issued, expired, used,
// scanned by someone else, or asked about by a browser that did not create it.
// Every one of those answers the same, so probing tokens learns nothing.
var ErrQRUnavailable = errors.New("qr code is not available")

// QR code states as the computer and the phone see them.
const (
	QRPending  = "pending"
	QRScanned  = "scanned"
	QRApproved = "approved"
	QRDeclined = "declined"
)

// QRTicket is what the computer gets when it asks for a code.
type QRTicket struct {
	Token      string
	PollSecret string
	ExpiresIn  time.Duration
}

// QRPhone is the signed-in person acting from the phone, from their token.
type QRPhone struct {
	UserID     string
	Username   string
	NGACNodeID string
	TenantID   string
	SessionID  string
	// IP is the address the phone's request came from. It is compared with the
	// computer's and never shown or kept.
	IP string
}

// QRScan is what the phone shows before the person approves.
type QRScan struct {
	Browser    string
	OS         string
	CreatedAt  time.Time
	TenantName string
	// SameNetwork is false when the computer asked from another network than the
	// phone scans from (or its network is unknown): the phone then warns.
	SameNetwork bool
}

// QRScanner is who scanned the code and where the computer would land, as the
// computer shows them before its person confirms.
type QRScanner struct {
	DisplayName string
	AvatarURL   string
	TenantName  string
}

// QRSession is the session handed to the computer once the phone approved and
// the person at the computer confirmed.
type QRSession struct {
	Token      string
	SessionID  string
	UserID     string
	Username   string
	NGACNodeID string
	TenantID   string
	Email      string
	Phone      string
	UnionID    string

	EmailVerified bool
	NeedsProfile  bool
}

// QRPoll is the computer's view of its code.
type QRPoll struct {
	Status  string
	Scanner *QRScanner // set once scanned
}

type qrState struct {
	Status    string    `json:"status"`
	PollHash  string    `json:"poll_hash"`
	Browser   string    `json:"browser,omitempty"`
	OS        string    `json:"os,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	// Network is the computer's network prefix (IPv4 /24, IPv6 /64), only for
	// comparing with the phone's; empty when the address was not usable.
	Network string `json:"network,omitempty"`

	ScannerUserID     string `json:"scanner_user_id,omitempty"`
	ScannerTenant     string `json:"scanner_tenant,omitempty"`
	ScannerName       string `json:"scanner_name,omitempty"`
	ScannerAvatar     string `json:"scanner_avatar,omitempty"`
	ScannerTenantName string `json:"scanner_tenant_name,omitempty"`
}

// qrNetwork is the network an address belongs to for the "same network" check:
// IPv4 /24, IPv6 /64. Offices and homes reach the server from one address or a
// few neighbouring ones; anything else is another network.
func qrNetwork(ip string) string {
	addr := net.ParseIP(strings.TrimSpace(ip))
	if addr == nil {
		return ""
	}
	if v4 := addr.To4(); v4 != nil {
		return v4.Mask(net.CIDRMask(24, 32)).String() + "/24"
	}
	return addr.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

func qrKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return qrLoginKeyPrefix + hex.EncodeToString(sum[:])
}

func qrHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func qrSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// CreateQRLogin issues a new code for the browser described by userAgent,
// asking from ip.
func (s *Service) CreateQRLogin(ctx context.Context, userAgent, ip string) (QRTicket, error) {
	if s.rdb == nil {
		return QRTicket{}, ErrQRUnavailable
	}
	token, err := qrSecret()
	if err != nil {
		return QRTicket{}, fmt.Errorf("qr token: %w", err)
	}
	poll, err := qrSecret()
	if err != nil {
		return QRTicket{}, fmt.Errorf("qr poll secret: %w", err)
	}
	browser, os := ParseUserAgent(userAgent)
	st := qrState{
		Status: QRPending, PollHash: qrHash(poll), Browser: browser, OS: os,
		CreatedAt: time.Now().UTC(), Network: qrNetwork(ip),
	}
	payload, err := json.Marshal(st)
	if err != nil {
		return QRTicket{}, fmt.Errorf("encode qr state: %w", err)
	}
	if err := s.rdb.Set(ctx, qrKey(token), payload, QRLoginTTL).Err(); err != nil {
		return QRTicket{}, fmt.Errorf("%w: store qr state: %v", ErrUnavailable, err)
	}
	return QRTicket{Token: token, PollSecret: poll, ExpiresIn: QRLoginTTL}, nil
}

// errQRNoChange tells qrUpdate to leave the state as it is.
var errQRNoChange = errors.New("qr state unchanged")

// qrUpdate reads the state of token and lets change rewrite it (or, returning
// del, remove it) in one optimistic transaction, so two phones or two polls
// racing on the same code cannot both win. The key keeps its expiry.
func (s *Service) qrUpdate(ctx context.Context, token string, change func(*qrState) (del bool, err error)) error {
	if s.rdb == nil || token == "" {
		return ErrQRUnavailable
	}
	key := qrKey(token)
	var outcome error
	txErr := s.rdb.Watch(ctx, func(tx *redis.Tx) error {
		payload, err := tx.Get(ctx, key).Bytes()
		if errors.Is(err, redis.Nil) {
			outcome = ErrQRUnavailable
			return nil
		}
		if err != nil {
			return err
		}
		var st qrState
		if json.Unmarshal(payload, &st) != nil {
			outcome = ErrQRUnavailable
			return nil
		}
		del, err := change(&st)
		if errors.Is(err, errQRNoChange) {
			return nil
		}
		if err != nil {
			outcome = err
			return nil
		}
		_, err = tx.TxPipelined(ctx, func(p redis.Pipeliner) error {
			if del {
				p.Del(ctx, key)
				return nil
			}
			next, err := json.Marshal(st)
			if err != nil {
				return err
			}
			p.SetArgs(ctx, key, next, redis.SetArgs{KeepTTL: true})
			return nil
		})
		return err
	}, key)
	switch {
	case errors.Is(txErr, redis.TxFailedErr):
		// Someone else changed the code in between: whatever they did wins.
		return ErrQRUnavailable
	case txErr != nil:
		return fmt.Errorf("%w: qr state: %v", ErrUnavailable, txErr)
	}
	return outcome
}

// qrPollerOwns refuses anyone but the page that created the code.
func qrPollerOwns(st *qrState, pollSecret string) error {
	if pollSecret == "" || subtle.ConstantTimeCompare([]byte(qrHash(pollSecret)), []byte(st.PollHash)) != 1 {
		return ErrQRUnavailable
	}
	return nil
}

// PollQRLogin is the computer asking how its code is doing. An approval is
// reported with who approved it and where the computer would land, and waits
// for the person at the computer to confirm (ClaimQRLogin) or refuse
// (CancelQRLogin). Only the page holding pollSecret is answered.
func (s *Service) PollQRLogin(ctx context.Context, token, pollSecret string) (*QRPoll, error) {
	var out QRPoll
	err := s.qrUpdate(ctx, token, func(st *qrState) (bool, error) {
		if err := qrPollerOwns(st, pollSecret); err != nil {
			return false, err
		}
		out.Status = st.Status
		if st.ScannerUserID != "" {
			out.Scanner = &QRScanner{DisplayName: st.ScannerName, AvatarURL: st.ScannerAvatar, TenantName: st.ScannerTenantName}
		}
		if st.Status == QRDeclined {
			// Told once; the code is finished either way.
			return true, nil
		}
		return false, errQRNoChange
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ClaimQRLogin is the person at the computer confirming "continue as …" after
// the phone approved. It mints the session and removes the code in one step, so
// the session is handed out once and only to the page holding pollSecret; if
// the user cannot be read the code stays and the page can try again.
func (s *Service) ClaimQRLogin(ctx context.Context, token, pollSecret string) (*QRSession, error) {
	var out *QRSession
	err := s.qrUpdate(ctx, token, func(st *qrState) (bool, error) {
		if err := qrPollerOwns(st, pollSecret); err != nil {
			return false, err
		}
		if st.Status != QRApproved {
			return false, ErrQRUnavailable
		}
		session, err := s.qrSession(ctx, *st)
		if err != nil {
			return false, err
		}
		out = session
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CancelQRLogin is "Không phải tôi" on the computer: the code is removed
// whatever state it is in, so nothing approved on it can be collected.
func (s *Service) CancelQRLogin(ctx context.Context, token, pollSecret string) error {
	return s.qrUpdate(ctx, token, func(st *qrState) (bool, error) {
		if err := qrPollerOwns(st, pollSecret); err != nil {
			return false, err
		}
		return true, nil
	})
}

// qrSession mints the computer's session for an approved code.
func (s *Service) qrSession(ctx context.Context, st qrState) (*QRSession, error) {
	u, err := s.store.GetUserByID(ctx, st.ScannerUserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if u == nil {
		return nil, ErrQRUnavailable
	}
	token, sessionID, err := auth.GenerateToken(u.ID, u.Username, u.NGACNodeID, st.ScannerTenant)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	return &QRSession{
		Token: token, SessionID: sessionID,
		UserID: u.ID, Username: u.Username, NGACNodeID: u.NGACNodeID, TenantID: st.ScannerTenant,
		Email: u.Email, Phone: u.Phone, UnionID: u.UnionID,
		EmailVerified: u.EmailVerified, NeedsProfile: !u.ProfileCompleted,
	}, nil
}

// ScanQRLogin records that the person on the phone opened the code and tells
// the phone which computer is asking. The phone must be an active member of
// the tenant it is using, since that is where the computer would land. Scanning
// again from the same person is harmless; anyone else is refused.
func (s *Service) ScanQRLogin(ctx context.Context, token string, phone QRPhone) (*QRScan, error) {
	if s.rdb == nil || token == "" {
		return nil, ErrQRUnavailable
	}
	// Refuse unknown codes before asking anyone about membership, so a probe
	// for tokens costs one Redis read.
	if n, err := s.rdb.Exists(ctx, qrKey(token)).Result(); err != nil {
		return nil, fmt.Errorf("%w: qr state: %v", ErrUnavailable, err)
	} else if n == 0 {
		return nil, ErrQRUnavailable
	}
	tenantName, err := s.qrMemberTenant(ctx, phone)
	if err != nil {
		return nil, err
	}
	user, _, err := s.GetMe(ctx, phone.UserID, "")
	if err != nil {
		return nil, err
	}
	name := user.DisplayName
	if name == "" {
		name = user.Username
	}

	var out QRScan
	err = s.qrUpdate(ctx, token, func(st *qrState) (bool, error) {
		switch {
		case st.Status == QRPending:
		case st.Status == QRScanned && st.ScannerUserID == phone.UserID:
		default:
			return false, ErrQRUnavailable
		}
		st.Status = QRScanned
		st.ScannerUserID, st.ScannerTenant = phone.UserID, phone.TenantID
		st.ScannerName, st.ScannerAvatar, st.ScannerTenantName = name, user.AvatarURL, tenantName
		phoneNet := qrNetwork(phone.IP)
		out = QRScan{
			Browser: st.Browser, OS: st.OS, CreatedAt: st.CreatedAt, TenantName: tenantName,
			SameNetwork: st.Network != "" && st.Network == phoneNet,
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ApproveQRLogin lets the computer in. Membership is asked again: the person
// may have been removed from the tenant since they scanned.
func (s *Service) ApproveQRLogin(ctx context.Context, token string, phone QRPhone) error {
	if err := s.qrScannedBy(ctx, token, phone); err != nil {
		return err
	}
	if _, err := s.qrMemberTenant(ctx, phone); err != nil {
		return err
	}
	var approved qrState
	err := s.qrUpdate(ctx, token, func(st *qrState) (bool, error) {
		if st.Status != QRScanned || st.ScannerUserID != phone.UserID || st.ScannerTenant != phone.TenantID {
			return false, ErrQRUnavailable
		}
		st.Status = QRApproved
		approved = *st
		return false, nil
	})
	if err != nil {
		return err
	}
	slog.Info("audit: qr sign-in approved from a phone",
		"user_id", phone.UserID, "tenant_id", phone.TenantID, "approving_session_id", phone.SessionID,
		"browser", approved.Browser, "os", approved.OS,
		"same_network", approved.Network != "" && approved.Network == qrNetwork(phone.IP))
	return nil
}

// DeclineQRLogin refuses the computer. Only the person who scanned may.
func (s *Service) DeclineQRLogin(ctx context.Context, token string, phone QRPhone) error {
	return s.qrUpdate(ctx, token, func(st *qrState) (bool, error) {
		if st.Status != QRScanned || st.ScannerUserID != phone.UserID {
			return false, ErrQRUnavailable
		}
		st.Status = QRDeclined
		return false, nil
	})
}

// qrScannedBy refuses a code that is not waiting for this person's approval,
// before membership is asked about.
func (s *Service) qrScannedBy(ctx context.Context, token string, phone QRPhone) error {
	return s.qrUpdate(ctx, token, func(st *qrState) (bool, error) {
		if st.Status != QRScanned || st.ScannerUserID != phone.UserID {
			return false, ErrQRUnavailable
		}
		return false, errQRNoChange
	})
}

// qrMemberTenant is the name of the phone's tenant when the phone's session is
// still live and its person an active member of the tenant: ErrQRSessionEnded
// for an ended session, ErrAccessDenied for anyone not a member.
func (s *Service) qrMemberTenant(ctx context.Context, phone QRPhone) (string, error) {
	live, err := s.refresh.SessionLive(ctx, phone.UserID, phone.SessionID)
	if err != nil {
		return "", err
	}
	if !live {
		return "", ErrQRSessionEnded
	}
	if phone.TenantID == "" {
		return "", ErrAccessDenied
	}
	row, err := s.activeMemberRow(ctx, phone.TenantID, phone.UserID, phone.NGACNodeID)
	if err != nil {
		return "", err
	}
	if row == nil {
		return "", nil
	}
	return row.TenantName, nil
}

// ParseUserAgent names the browser and operating system of a User-Agent header
// for the phone to show ("Chrome", "Windows"). Unknown parts are empty.
func ParseUserAgent(ua string) (browser, os string) {
	switch {
	case strings.Contains(ua, "Edg/"), strings.Contains(ua, "EdgA/"), strings.Contains(ua, "EdgiOS/"):
		browser = "Edge"
	case strings.Contains(ua, "OPR/"), strings.Contains(ua, "Opera"):
		browser = "Opera"
	case strings.Contains(ua, "Firefox/"), strings.Contains(ua, "FxiOS/"):
		browser = "Firefox"
	case strings.Contains(ua, "Chrome/"), strings.Contains(ua, "CriOS/"):
		browser = "Chrome"
	case strings.Contains(ua, "Safari/") && strings.Contains(ua, "Version/"):
		browser = "Safari"
	}
	switch {
	case strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPad"):
		os = "iOS"
	case strings.Contains(ua, "Android"):
		os = "Android"
	case strings.Contains(ua, "CrOS"):
		os = "ChromeOS"
	case strings.Contains(ua, "Windows"):
		os = "Windows"
	case strings.Contains(ua, "Macintosh"), strings.Contains(ua, "Mac OS X"):
		os = "macOS"
	case strings.Contains(ua, "Linux"):
		os = "Linux"
	}
	return browser, os
}
