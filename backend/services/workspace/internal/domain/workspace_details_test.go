package domain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/workspace/internal/domain"
)

func ptr(s string) *string { return &s }

func TestWorkspaceDetails_AMemberReadsAndLearnsWhetherTheyMayEdit(t *testing.T) {
	f := newFixture(t)
	f.wsStore.ws[ws1].Desc = "Đối soát"

	d, err := f.svc.WorkspaceDetails(context.Background(), member, ws1)
	require.NoError(t, err)
	assert.Equal(t, "Acme", d.Name)
	assert.Equal(t, "Đối soát", d.Description)
	assert.False(t, d.CanManage, "a plain member may read but not edit")

	d, err = f.svc.WorkspaceDetails(context.Background(), manager, ws1)
	require.NoError(t, err)
	assert.True(t, d.CanManage)
}

func TestWorkspaceDetails_NonMembersAreDenied(t *testing.T) {
	for _, caller := range []string{outsider, otherOwner, ""} {
		t.Run(caller, func(t *testing.T) {
			f := newFixture(t)
			_, err := f.svc.WorkspaceDetails(context.Background(), caller, ws1)
			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrAccessDenied)
		})
	}
}

func TestUpdateWorkspaceDetails_ManageSavesTrimmedValues(t *testing.T) {
	f := newFixture(t)

	d, err := f.svc.UpdateWorkspaceDetails(context.Background(), manager, ws1, ptr("  Khối Vận hành  "), ptr(" Đối soát và thanh toán. "))

	require.NoError(t, err)
	assert.Equal(t, "Khối Vận hành", d.Name)
	assert.Equal(t, "Đối soát và thanh toán.", d.Description)
	assert.Equal(t, "Khối Vận hành", f.wsStore.ws[ws1].Name)
	assert.Equal(t, "Globex", f.wsStore.ws[ws2].Name, "another workspace is untouched")
}

func TestUpdateWorkspaceDetails_AFieldLeftOutIsKept(t *testing.T) {
	f := newFixture(t)
	f.wsStore.ws[ws1].Desc = "keep me"

	d, err := f.svc.UpdateWorkspaceDetails(context.Background(), manager, ws1, ptr("New"), nil)

	require.NoError(t, err)
	assert.Equal(t, "keep me", d.Description)
}

func TestUpdateWorkspaceDetails_RejectsInvalidInputBeforeWriting(t *testing.T) {
	cases := map[string]struct{ name, desc *string }{
		"empty name":        {name: ptr("   ")},
		"name too long":     {name: ptr(strings.Repeat("a", domain.MaxWorkspaceNameRunes+1))},
		"NUL in name":       {name: ptr("a\x00")},
		"desc too long":     {desc: ptr(strings.Repeat("a", domain.MaxWorkspaceDescriptionRunes+1))},
		"nothing to change": {},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			_, err := f.svc.UpdateWorkspaceDetails(context.Background(), manager, ws1, c.name, c.desc)
			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.False(t, f.mutated())
		})
	}
}
