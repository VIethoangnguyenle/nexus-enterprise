package domain_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ngac-platform/services/auth/internal/domain"
)

func ptr(s string) *string { return &s }

func TestUpdateProfile_Validation(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	u := w.addUser("pf@example.test")

	vietnamese80 := strings.Repeat("Ệ", 80) // 80 runes, 240 bytes: the limit counts people-visible characters
	cases := []struct {
		name string
		in   domain.ProfileUpdateInput
		ok   bool
	}{
		{"nothing to change", domain.ProfileUpdateInput{}, false},
		{"empty display name", domain.ProfileUpdateInput{DisplayName: ptr("")}, false},
		{"blank display name", domain.ProfileUpdateInput{DisplayName: ptr("   \t ")}, false},
		{"81-rune display name", domain.ProfileUpdateInput{DisplayName: ptr(strings.Repeat("a", 81))}, false},
		{"80 Vietnamese runes", domain.ProfileUpdateInput{DisplayName: ptr(vietnamese80)}, true},
		{"newline in the name", domain.ProfileUpdateInput{DisplayName: ptr("An\nAdmin")}, false},
		{"NUL in the name", domain.ProfileUpdateInput{DisplayName: ptr("An\x00")}, false},
		{"title too long", domain.ProfileUpdateInput{Title: ptr(strings.Repeat("t", 121))}, false},
		{"clearing a title is allowed", domain.ProfileUpdateInput{Title: ptr("")}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.UpdateProfile(context.Background(), u.ID, tc.in)
			if tc.ok && err != nil {
				t.Fatalf("want success, got %v", err)
			}
			if !tc.ok && !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("want ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestUpdateProfile_RejectedInputWritesNothing(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	u := w.addUser("pf2@example.test")

	_ = svc.UpdateProfile(context.Background(), u.ID, domain.ProfileUpdateInput{DisplayName: ptr("Ok Name"), Title: ptr(strings.Repeat("t", 121))})

	got, _ := svc.GetUserByID(context.Background(), u.ID)
	if got.DisplayName != "Existing" {
		t.Errorf("a refused update must not apply its valid half, display name is %q", got.DisplayName)
	}
	me, _, _ := svc.GetMe(context.Background(), u.ID, "")
	if !me.NeedsProfile {
		t.Error("a refused update must not mark the profile done")
	}
}

func TestUpdateProfile_TrimsAndMarksTheProfileDone(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	u := w.addUser("pf3@example.test")

	me, _, err := svc.GetMe(context.Background(), u.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if !me.NeedsProfile {
		t.Fatal("a new account owes a profile")
	}

	if err := svc.UpdateProfile(context.Background(), u.ID, domain.ProfileUpdateInput{DisplayName: ptr("  Phạm Thuý An  ")}); err != nil {
		t.Fatal(err)
	}
	me, _, _ = svc.GetMe(context.Background(), u.ID, "")
	if me.DisplayName != "Phạm Thuý An" {
		t.Errorf("display name = %q, want it trimmed", me.DisplayName)
	}
	if me.NeedsProfile {
		t.Error("saving the profile ends the step")
	}
}

func TestUpdateProfile_UnknownAccountIsNotFound(t *testing.T) {
	svc := newFakeWorld().service(t)
	err := svc.UpdateProfile(context.Background(), "nobody", domain.ProfileUpdateInput{DisplayName: ptr("x")})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if err := svc.UpdateProfile(context.Background(), "", domain.ProfileUpdateInput{DisplayName: ptr("x")}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("empty user id: err = %v, want ErrInvalidInput", err)
	}
}

func TestGetMe_ReportsWhetherTheAddressIsProved(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	u := w.addUser("proof@example.test")

	me, _, _ := svc.GetMe(context.Background(), u.ID, "")
	if me.EmailVerified {
		t.Error("an address nobody proved is not verified")
	}
	if _, err := w.MarkEmailVerified(context.Background(), u.ID); err != nil {
		t.Fatal(err)
	}
	me, _, _ = svc.GetMe(context.Background(), u.ID, "")
	if !me.EmailVerified {
		t.Error("a proved address is reported verified")
	}
}
