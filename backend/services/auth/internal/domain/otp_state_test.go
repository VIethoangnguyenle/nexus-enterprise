package domain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"ngac-platform/services/auth/internal/domain"
)

func TestOTP_WrongCodeSaysHowManyTriesAreLeft(t *testing.T) {
	svc, _, _ := otpService(t, &domain.OTPOptions{FixedCode: "999999"})
	sid, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []int{4, 3, 2, 1, 0} {
		_, err := svc.VerifyOTP(context.Background(), sid, "123456")
		if !errors.Is(err, domain.ErrOTPInvalid) {
			t.Fatalf("err = %v, want ErrOTPInvalid", err)
		}
		left, ok := domain.OTPAttemptsLeft(err)
		if !ok || left != want {
			t.Fatalf("attempts left = %d (%v), want %d", left, ok, want)
		}
	}
	if _, err := svc.VerifyOTP(context.Background(), sid, "999999"); !errors.Is(err, domain.ErrTooManyAttempts) {
		t.Fatalf("err = %v, want ErrTooManyAttempts", err)
	}
}

func TestOTP_OtherFailuresCarryNoAttemptCount(t *testing.T) {
	svc, _, _ := otpService(t, &domain.OTPOptions{FixedCode: "999999"})
	_, err := svc.VerifyOTP(context.Background(), "no-such-session", "999999")
	if _, ok := domain.OTPAttemptsLeft(err); ok {
		t.Error("an expired or unknown session has no attempts to count")
	}
	if !errors.Is(err, domain.ErrOTPExpired) {
		t.Errorf("err = %v, want ErrOTPExpired", err)
	}
}

func TestOTP_RateLimitSaysWhenToComeBack(t *testing.T) {
	svc, _, _ := otpService(t, &domain.OTPOptions{FixedCode: "999999"})
	email := uniqueEmail()
	for i := 0; i < 5; i++ {
		if _, err := svc.RequestOTP(context.Background(), email, "email"); err != nil {
			t.Fatal(err)
		}
	}
	_, err := svc.RequestOTP(context.Background(), email, "email")
	if !errors.Is(err, domain.ErrOTPRateLimited) {
		t.Fatalf("err = %v, want ErrOTPRateLimited", err)
	}
	d, ok := domain.RetryAfter(err)
	if !ok || d <= 0 || d > 15*time.Minute {
		t.Errorf("retry after = %v (%v), want within the 15 minute window", d, ok)
	}
}

func TestOTP_ResultReportsAccountState(t *testing.T) {
	t.Run("fixed code: a new account is unverified and owes a profile", func(t *testing.T) {
		svc, _, _ := otpService(t, &domain.OTPOptions{FixedCode: "999999"})
		sid, _ := svc.RequestOTP(context.Background(), uniqueEmail(), "email")
		res, err := svc.VerifyOTP(context.Background(), sid, "999999")
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsNewUser || res.EmailVerified || !res.NeedsProfile {
			t.Errorf("result = %+v", res)
		}
	})

	t.Run("delivered code: a new account is verified", func(t *testing.T) {
		sender := &verifyingSender{}
		svc, _, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: sender})
		sid, _ := svc.RequestOTP(context.Background(), uniqueEmail(), "email")
		res, err := svc.VerifyOTP(context.Background(), sid, sender.last())
		if err != nil {
			t.Fatal(err)
		}
		if !res.EmailVerified || !res.NeedsProfile {
			t.Errorf("result = %+v", res)
		}
	})

	t.Run("delivered code proves an existing address in the same response", func(t *testing.T) {
		sender := &verifyingSender{}
		svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: sender})
		addr := uniqueEmail()
		existing := w.addUser(addr)
		sid, _ := svc.RequestOTP(context.Background(), addr, "email")
		res, err := svc.VerifyOTP(context.Background(), sid, sender.last())
		if err != nil {
			t.Fatal(err)
		}
		if res.IsNewUser || !res.EmailVerified {
			t.Errorf("result = %+v, want existing + verified", res)
		}
		if res.UserID != existing.ID {
			t.Error("the same account")
		}
	})

	t.Run("a returning person who finished their profile owes nothing", func(t *testing.T) {
		svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "999999"})
		addr := uniqueEmail()
		u := w.addUser(addr)
		_ = svc.UpdateProfile(context.Background(), u.ID, domain.ProfileUpdateInput{DisplayName: ptr("An")})
		sid, _ := svc.RequestOTP(context.Background(), addr, "email")
		res, err := svc.VerifyOTP(context.Background(), sid, "999999")
		if err != nil {
			t.Fatal(err)
		}
		if res.NeedsProfile {
			t.Errorf("result = %+v", res)
		}
	})

	t.Run("a phone account has no address to prove", func(t *testing.T) {
		sender := &verifyingSender{}
		svc, _, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: sender})
		sid, _ := svc.RequestOTP(context.Background(), "09"+time.Now().Format("150405999")[:8], "phone")
		res, err := svc.VerifyOTP(context.Background(), sid, sender.last())
		if err != nil {
			t.Fatal(err)
		}
		if res.EmailVerified {
			t.Error("a phone number is not an email address")
		}
	})
}

// The refresh token must name the workspace the access token does: after a page
// reload the app trades the cookie for a new access token, and one scoped to
// nothing would leave every per-workspace service without its workspace.
func TestOTP_ResultNamesTheWorkspaceTheTokenIsScopedTo(t *testing.T) {
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "999999"})
	addr := uniqueEmail()
	u := w.addUser(addr)
	home := w.addTenant("Khối", "")
	_ = w.InsertTenantUser(context.Background(), home, u.ID, "owner", "active", u.NGACNodeID)

	sid, _ := svc.RequestOTP(context.Background(), addr, "email")
	res, err := svc.VerifyOTP(context.Background(), sid, "999999")
	if err != nil {
		t.Fatal(err)
	}
	if res.TenantID != home {
		t.Errorf("TenantID = %q, want %q", res.TenantID, home)
	}
	if got := tenantOf(t, res.Token); got != home {
		t.Errorf("the token's tenant = %q, want %q", got, home)
	}

	fresh := uniqueEmail()
	sid, _ = svc.RequestOTP(context.Background(), fresh, "email")
	res, err = svc.VerifyOTP(context.Background(), sid, "999999")
	if err != nil {
		t.Fatal(err)
	}
	if res.TenantID == "" || res.TenantID != tenantOf(t, res.Token) {
		t.Errorf("a new account's personal workspace: TenantID %q, token %q", res.TenantID, tenantOf(t, res.Token))
	}
}
