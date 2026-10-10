package domain_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ngac-platform/services/auth/internal/domain"
)

// Proving the address of the account you are signed in to must prove it and
// nothing else: no new session, no switch of account, no change of tenant. Only
// a code a real sender delivered to the address, or a Google account whose own
// verified email is that address, counts.

func realSender() *verifyingSender { return &verifyingSender{} }

func TestEmailCode_ProvesTheSignedInAccountsAddress(t *testing.T) {
	sender := realSender()
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: sender})
	addr := uniqueEmail()
	u := w.addUser(addr)
	ctx := context.Background()

	if _, err := svc.RequestEmailVerification(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if sender.last() == "" {
		t.Fatal("the code was not delivered")
	}
	if w.emailVerified(u.ID) {
		t.Fatal("requesting a code must not verify anything")
	}
	if err := svc.ConfirmEmailVerification(ctx, u.ID, "current-session", sender.last()); err != nil {
		t.Fatal(err)
	}
	if !w.emailVerified(u.ID) {
		t.Error("the right code proves the address")
	}
}

func TestEmailCode_KeepsTheCurrentSessionAndEndsTheOthers(t *testing.T) {
	sender := realSender()
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: sender})
	addr := uniqueEmail()
	u := w.addUser(addr)
	u.Password = "set-before-the-proof"
	ctx := context.Background()

	mine, err := svc.AttachRefreshToken(ctx, domain.RefreshIdentity{UserID: u.ID, Username: "u", NGACNodeID: "n", SessionID: "mine"})
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := svc.AttachRefreshToken(ctx, domain.RefreshIdentity{UserID: u.ID, Username: "u", NGACNodeID: "n", SessionID: "theirs"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.RequestEmailVerification(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.ConfirmEmailVerification(ctx, u.ID, "mine", sender.last()); err != nil {
		t.Fatal(err)
	}

	if _, _, err := svc.RefreshSession(ctx, theirs); err == nil {
		t.Error("a session that existed before the address was proved must end: whoever opened it never held the mailbox")
	}
	if _, _, err := svc.RefreshSession(ctx, mine); err != nil {
		t.Errorf("the session that proved the address must go on: %v", err)
	}
	if passwordOf(w, u.ID) != "" {
		t.Error("a credential set up before the proof is dropped")
	}
}

func TestEmailCode_NeverHandsOutASession(t *testing.T) {
	// The domain call returns only an error: there is no token to hand out.
	var _ func(context.Context, string, string, string) error = (*domain.Service)(nil).ConfirmEmailVerification
}

func TestEmailCode_UnavailableUnlessACodeCouldProveIt(t *testing.T) {
	for name, opts := range map[string]domain.OTPOptions{
		"the fixed test code":  {FixedCode: "999999"},
		"the log sender":       {Sender: domain.LogSender{}},
		"no way to send codes": {},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("APP_ENV", "dev")
			svc, w, _ := otpService(t, &opts)
			u := w.addUser(uniqueEmail())
			_, err := svc.RequestEmailVerification(context.Background(), u.ID)
			if !errors.Is(err, domain.ErrVerificationUnavailable) {
				t.Fatalf("err = %v, want ErrVerificationUnavailable", err)
			}
			if w.emailVerified(u.ID) {
				t.Fatal("verified without proof")
			}
		})
	}
}

func TestEmailCode_Refusals(t *testing.T) {
	sender := realSender()
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: sender})
	ctx := context.Background()

	t.Run("an account with no address", func(t *testing.T) {
		u := w.addUser("")
		u.Email = ""
		if _, err := svc.RequestEmailVerification(ctx, u.ID); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("err = %v, want ErrInvalidInput", err)
		}
	})
	t.Run("an unknown account", func(t *testing.T) {
		if _, err := svc.RequestEmailVerification(ctx, "ghost"); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
	t.Run("an address that is already proved", func(t *testing.T) {
		u := w.addUser(uniqueEmail())
		_, _ = w.MarkEmailVerified(ctx, u.ID)
		if _, err := svc.RequestEmailVerification(ctx, u.ID); !errors.Is(err, domain.ErrAlreadyVerified) {
			t.Errorf("err = %v, want ErrAlreadyVerified", err)
		}
	})
	t.Run("a code was never asked for", func(t *testing.T) {
		u := w.addUser(uniqueEmail())
		if err := svc.ConfirmEmailVerification(ctx, u.ID, "s", "123456"); !errors.Is(err, domain.ErrOTPExpired) {
			t.Errorf("err = %v, want ErrOTPExpired", err)
		}
		if w.emailVerified(u.ID) {
			t.Error("verified without a code")
		}
	})
	t.Run("the code is not a code", func(t *testing.T) {
		u := w.addUser(uniqueEmail())
		if _, err := svc.RequestEmailVerification(ctx, u.ID); err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{"", "12345", "1234567", "abcdef"} {
			if err := svc.ConfirmEmailVerification(ctx, u.ID, "s", bad); err == nil {
				t.Errorf("%q was accepted", bad)
			}
		}
	})
}

func TestEmailCode_WrongCodesAreCountedAndEventuallyEndTheAttempt(t *testing.T) {
	sender := realSender()
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: sender})
	u := w.addUser(uniqueEmail())
	ctx := context.Background()
	if _, err := svc.RequestEmailVerification(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	right := sender.last()
	wrong := "000000"
	if right == wrong {
		wrong = "111111"
	}

	for _, want := range []int{4, 3, 2, 1, 0} {
		err := svc.ConfirmEmailVerification(ctx, u.ID, "s", wrong)
		if !errors.Is(err, domain.ErrOTPInvalid) {
			t.Fatalf("err = %v, want ErrOTPInvalid", err)
		}
		if left, ok := domain.OTPAttemptsLeft(err); !ok || left != want {
			t.Fatalf("attempts left = %d (%v), want %d", left, ok, want)
		}
	}
	if err := svc.ConfirmEmailVerification(ctx, u.ID, "s", right); !errors.Is(err, domain.ErrTooManyAttempts) {
		t.Fatalf("after five wrong codes even the right one fails, got %v", err)
	}
	if w.emailVerified(u.ID) {
		t.Fatal("verified after the attempts ran out")
	}
	if err := svc.ConfirmEmailVerification(ctx, u.ID, "s", right); !errors.Is(err, domain.ErrOTPExpired) {
		t.Fatalf("the attempt is over: err = %v, want ErrOTPExpired", err)
	}
}

func TestEmailCode_ACodeBelongsToTheAccountThatAskedForIt(t *testing.T) {
	sender := realSender()
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: sender})
	ctx := context.Background()
	victim := w.addUser(uniqueEmail())
	attacker := w.addUser(uniqueEmail())

	if _, err := svc.RequestEmailVerification(ctx, victim.ID); err != nil {
		t.Fatal(err)
	}
	stolen := sender.last()
	// The attacker has their own session and the victim's code (say, over a shoulder).
	if err := svc.ConfirmEmailVerification(ctx, attacker.ID, "s", stolen); err == nil {
		t.Fatal("a code issued to another account was accepted")
	}
	if w.emailVerified(attacker.ID) || w.emailVerified(victim.ID) {
		t.Fatal("nothing may be verified by a code that belongs to someone else")
	}
}

func TestEmailCode_CannotBeReusedAndDiesWithAChangedAddress(t *testing.T) {
	sender := realSender()
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: sender})
	ctx := context.Background()

	u := w.addUser(uniqueEmail())
	if _, err := svc.RequestEmailVerification(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	code := sender.last()
	if err := svc.ConfirmEmailVerification(ctx, u.ID, "s", code); err != nil {
		t.Fatal(err)
	}
	other := w.addUser(uniqueEmail())
	// Same code, spent: nobody can use it again.
	if err := svc.ConfirmEmailVerification(ctx, other.ID, "s", code); err == nil {
		t.Error("a spent code was accepted")
	}

	// The address changed after the code was sent: the code was for the old one.
	v := w.addUser(uniqueEmail())
	if _, err := svc.RequestEmailVerification(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	w.users[v.ID].Email = uniqueEmail()
	w.mu.Unlock()
	if err := svc.ConfirmEmailVerification(ctx, v.ID, "s", sender.last()); err == nil {
		t.Error("a code for the old address proved the new one")
	}
}

func TestEmailCode_RequestsAreRateLimited(t *testing.T) {
	sender := realSender()
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: sender})
	u := w.addUser(uniqueEmail())
	for i := 0; i < 5; i++ {
		if _, err := svc.RequestEmailVerification(context.Background(), u.ID); err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
	}
	_, err := svc.RequestEmailVerification(context.Background(), u.ID)
	if !errors.Is(err, domain.ErrOTPRateLimited) {
		t.Fatalf("6th request: %v, want ErrOTPRateLimited", err)
	}
	if _, ok := domain.RetryAfter(err); !ok {
		t.Error("a refusal says when to come back")
	}
}

func TestEmailCode_ADeliveryFailureLeavesNoLiveCode(t *testing.T) {
	sender := realSender()
	sender.err = errors.New("smtp down")
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: sender})
	u := w.addUser(uniqueEmail())
	if _, err := svc.RequestEmailVerification(context.Background(), u.ID); err == nil {
		t.Fatal("expected the delivery failure")
	}
	if err := svc.ConfirmEmailVerification(context.Background(), u.ID, "s", sender.last()); err == nil {
		t.Fatal("a code that was never delivered must not work")
	}
}

// --- Google ---

func TestGoogleProof_AcceptsOnlyTheAccountsOwnVerifiedAddress(t *testing.T) {
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "999999"})
	ctx := context.Background()
	addr := uniqueEmail()
	u := w.addUser(addr)

	// Another Google account: its owner signed in as someone else.
	if err := svc.VerifyEmailWithGoogle(ctx, u.ID, "s", googleIdentity("sub-other", uniqueEmail(), "")); !errors.Is(err, domain.ErrEmailMismatch) {
		t.Fatalf("a different address: err = %v, want ErrEmailMismatch", err)
	}
	// Google did not verify the address it names.
	unverified := googleIdentity("sub-u", addr, "")
	unverified.EmailVerified = false
	if err := svc.VerifyEmailWithGoogle(ctx, u.ID, "s", unverified); !errors.Is(err, domain.ErrEmailNotVerified) {
		t.Fatalf("unverified at Google: err = %v, want ErrEmailNotVerified", err)
	}
	if w.emailVerified(u.ID) {
		t.Fatal("nothing was proved yet")
	}

	// The same address, in another case: that is the same address.
	if err := svc.VerifyEmailWithGoogle(ctx, u.ID, "s", googleIdentity("sub-ok", " "+upper(addr)+" ", "")); err != nil {
		t.Fatal(err)
	}
	if !w.emailVerified(u.ID) {
		t.Error("the account's own address, verified by Google, proves it")
	}
}

func TestGoogleProof_DoesNotSignInOrLinkOrChangeAnyAccount(t *testing.T) {
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "999999"})
	ctx := context.Background()
	u := w.addUser(uniqueEmail())
	before := w.userCount()

	_ = svc.VerifyEmailWithGoogle(ctx, u.ID, "s", googleIdentity("sub-x", uniqueEmail(), ""))
	_ = svc.VerifyEmailWithGoogle(ctx, u.ID, "s", googleIdentity("sub-y", u.Email, ""))

	if w.userCount() != before {
		t.Error("an account was created")
	}
	if w.workspaceCount() != 0 {
		t.Error("a workspace was created")
	}
	w.mu.Lock()
	linked := len(w.identities)
	w.mu.Unlock()
	if linked != 0 {
		t.Error("a Google identity was linked: proving an address is not signing in")
	}
}

func TestGoogleProof_UnknownAccount(t *testing.T) {
	svc, _, _ := otpService(t, &domain.OTPOptions{FixedCode: "999999"})
	if err := svc.VerifyEmailWithGoogle(context.Background(), "ghost", "s", googleIdentity("s", "a@b.vn", "")); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func upper(s string) string { return strings.ToUpper(s) }
