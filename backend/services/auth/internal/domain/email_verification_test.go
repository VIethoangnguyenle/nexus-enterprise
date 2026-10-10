package domain_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
)

// An address is only a claim until its owner proves it. Everything that treats
// an address as identity (a workspace invitation) uses users.email_verified_at,
// so these pin which paths may set it and which may not.

// verifyingSender is a TEST-ONLY sender that stands for a real mail or SMS
// provider: it declares that its codes reach the owner of the address.
type verifyingSender struct{ recordingSender }

func (*verifyingSender) DeliversToOwner() bool { return true }

func TestOTP_StoresTheAddressNormalisedAndUnverified(t *testing.T) {
	svc, w, _ := otpService(t, nil)
	base := fmt.Sprintf("Eve.%d", time.Now().UnixNano())
	sid, err := svc.RequestOTP(context.Background(), "  "+base+"@Acme.COM ", "email")
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.VerifyOTP(context.Background(), sid, "999999")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.ToLower(base) + "@acme.com"
	if res.Email != want {
		t.Errorf("email = %q, want trimmed and lower-cased %q", res.Email, want)
	}
	if u := w.userByEmail(want); u == nil || u.Email != want {
		t.Errorf("stored email = %+v", u)
	}
	if w.emailVerified(res.UserID) {
		t.Error("the fixed test code proves nothing about the address")
	}
}

func TestOTP_CaseVariantOfAnExistingAddressIsTheSameAccount(t *testing.T) {
	svc, w, _ := otpService(t, nil)
	base := fmt.Sprintf("owner%d", time.Now().UnixNano())
	existing := w.addUser(base + "@acme.com")
	for _, variant := range []string{strings.ToUpper(base) + "@acme.com", "  " + base + "@Acme.com  "} {
		sid, err := svc.RequestOTP(context.Background(), variant, "email")
		if err != nil {
			t.Fatal(err)
		}
		res, err := svc.VerifyOTP(context.Background(), sid, "999999")
		if err != nil {
			t.Fatal(err)
		}
		if res.UserID != existing.ID || res.IsNewUser {
			t.Errorf("%q: signed in as %q (new=%v), want the existing account", variant, res.UserID, res.IsNewUser)
		}
	}
	if w.userCount() != 1 {
		t.Errorf("no account may be created for a case variant, have %d", w.userCount())
	}
}

func TestOTP_RefusesWhatIsNotAnAddress(t *testing.T) {
	svc, w, _ := otpService(t, nil)
	for _, in := range []string{"", "   ", "no-at-sign", "a@b", "a b@c.vn"} {
		if _, err := svc.RequestOTP(context.Background(), in, "email"); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%q: err = %v, want ErrInvalidInput", in, err)
		}
	}
	if w.userCount() != 0 {
		t.Error("no account may be created")
	}
}

func TestGoogle_VerifiedEmailMarksTheAccountVerified(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	if _, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-new", "New.Person@Acme.com", "")); err != nil {
		t.Fatal(err)
	}
	u := w.userByEmail("new.person@acme.com")
	if u == nil || !w.emailVerified(u.ID) {
		t.Fatalf("a new account from Google's verified email is verified: %+v", u)
	}
}

func TestGoogle_LinkingAnExistingAccountVerifiesItAndDropsTheUnverifiedPassword(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	existing := w.addUser("victim@acme.com")
	existing.Password = "attacker-hash"
	if _, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-v", "victim@acme.com", "")); err != nil {
		t.Fatal(err)
	}
	if !w.emailVerified(existing.ID) {
		t.Error("Google proved the address")
	}
	if got := w.userByEmail("victim@acme.com"); got.Password != "" {
		t.Error("the password set before the proof is dropped")
	}
}

func TestGoogle_UnverifiedEmailVerifiesNothing(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	id := googleIdentity("sub-x", "someone@acme.com", "")
	id.EmailVerified = false
	if _, err := svc.SignInWithGoogle(context.Background(), id); err == nil {
		t.Fatal("expected rejection")
	}
	if w.userCount() != 0 {
		t.Error("and no account")
	}
}

// --- one-time codes ---

func TestOTP_FixedTestCodeNeverVerifiesAnAddress(t *testing.T) {
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "999999"})
	knownAddr := uniqueEmail()
	existing := w.addUser(knownAddr)

	// A brand-new address through the fixed code: account created, not verified.
	fresh := uniqueEmail()
	sid, err := svc.RequestOTP(context.Background(), fresh, "email")
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.VerifyOTP(context.Background(), sid, "999999")
	if err != nil {
		t.Fatal(err)
	}
	if w.emailVerified(res.UserID) {
		t.Error("the fixed code proves nothing: a new account stays unverified")
	}

	// An existing account's address through the fixed code: still not verified.
	sid, _ = svc.RequestOTP(context.Background(), knownAddr, "email")
	if _, err := svc.VerifyOTP(context.Background(), sid, "999999"); err != nil {
		t.Fatal(err)
	}
	if w.emailVerified(existing.ID) {
		t.Error("the fixed code must not verify an existing account")
	}
}

func TestOTP_LogSenderNeverVerifies(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: domain.LogSender{}})
	// LogSender writes the code to the log: whoever reads the log is not the owner.
	loggedAddr := uniqueEmail()
	existing := w.addUser(loggedAddr)
	if _, err := svc.RequestOTP(context.Background(), loggedAddr, "email"); err != nil {
		t.Fatal(err)
	}
	if w.emailVerified(existing.ID) {
		t.Fatal("unreachable")
	}
	if svc.OTPProvesOwnership() {
		t.Error("log delivery is not proof of ownership")
	}
}

func TestOTP_CodeDeliveredByARealSenderVerifiesNewAndExistingAccounts(t *testing.T) {
	sender := &verifyingSender{}
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: sender})
	if !svc.OTPProvesOwnership() {
		t.Fatal("a sender that declares delivery to the owner proves ownership")
	}

	fresh := uniqueEmail()
	sid, _ := svc.RequestOTP(context.Background(), fresh, "email")
	res, err := svc.VerifyOTP(context.Background(), sid, sender.last())
	if err != nil {
		t.Fatal(err)
	}
	if !w.emailVerified(res.UserID) {
		t.Error("a new account verified through a delivered code is verified")
	}

	// A password account made earlier by someone else loses that password when the owner proves the address.
	preAddr := uniqueEmail()
	pre := w.addUser(preAddr)
	pre.Password = "attacker-hash"
	sid, _ = svc.RequestOTP(context.Background(), preAddr, "email")
	if _, err := svc.VerifyOTP(context.Background(), sid, sender.last()); err != nil {
		t.Fatal(err)
	}
	if !w.emailVerified(pre.ID) {
		t.Error("an existing account is verified")
	}
	if got := w.userByEmail(preAddr); got.Password != "" {
		t.Error("credentials set before the proof are dropped")
	}
}

func TestOTP_PhoneCodesNeverVerifyAnEmail(t *testing.T) {
	sender := &verifyingSender{}
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: sender})
	// Unique per run: OTP requests are rate-limited per identifier in Redis.
	phone := fmt.Sprintf("09%08d", time.Now().UnixNano()%100000000)
	sid, err := svc.RequestOTP(context.Background(), phone, "phone")
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.VerifyOTP(context.Background(), sid, sender.last())
	if err != nil {
		t.Fatal(err)
	}
	if w.emailVerified(res.UserID) {
		t.Error("a phone code says nothing about an email")
	}
	_ = auth.SetJWTSecret
}
