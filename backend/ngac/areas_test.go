package ngac_test

import (
	"slices"
	"testing"

	"ngac-platform/ngac"
)

// Every operation a screen can offer comes from the eight fixed operations.
func TestAreaOpsAreAmongTheEightOperations(t *testing.T) {
	known := ngac.AllOwnerOps()
	for _, a := range ngac.Areas() {
		ops := ngac.AreaOps(a)
		if len(ops) == 0 {
			t.Errorf("area %q offers no operation", a)
		}
		for _, op := range ops {
			if !slices.Contains(known, op) {
				t.Errorf("area %q offers %q, which is not an NGAC operation", a, op)
			}
		}
	}
}

// The answer is in the canonical order and has no duplicates, so a screen can
// render it as it is.
func TestAreaOpsAreCanonicalAndUnique(t *testing.T) {
	canonical := ngac.AllOwnerOps()
	for _, a := range ngac.Areas() {
		ops := ngac.AreaOps(a)
		last := -1
		for _, op := range ops {
			i := slices.Index(canonical, op)
			if i <= last {
				t.Errorf("area %q: %v is out of order or repeated at %q", a, ops, op)
			}
			last = i
		}
	}
}

// What an area offers is what services check there, no more and no less.
func TestAreaOpsPerArea(t *testing.T) {
	cases := map[ngac.Area][]string{
		ngac.AreaManagement: {ngac.OpManage, ngac.OpInvite},
		ngac.AreaDocuments:  {ngac.OpRead, ngac.OpWrite, ngac.OpShare},
		ngac.AreaChannels:   {ngac.OpRead, ngac.OpWrite, ngac.OpManage, ngac.OpInvite, ngac.OpCreateChannel},
		ngac.AreaAssets:     {ngac.OpRead, ngac.OpWrite, ngac.OpApprove, ngac.OpManage},
	}
	for a, want := range cases {
		got := ngac.AreaOps(a)
		if !slices.Equal(got, sorted(want)) {
			t.Errorf("area %q: got %v, want %v", a, got, sorted(want))
		}
	}
}

func sorted(ops []string) []string {
	canonical := ngac.AllOwnerOps()
	out := slices.Clone(ops)
	slices.SortFunc(out, func(a, b string) int { return slices.Index(canonical, a) - slices.Index(canonical, b) })
	return out
}

// Members hold share on Documents and on channel drives; the permissions
// screen must be able to show it, so both areas have to offer it.
func TestMemberGrantsFitTheirAreas(t *testing.T) {
	for _, op := range ngac.MemberDocumentOps() {
		if op == ngac.OpUpload {
			continue // granted, but no service checks it: the drive gates an upload on write
		}
		if !slices.Contains(ngac.AreaOps(ngac.AreaDocuments), op) {
			t.Errorf("members hold %q on Documents but the documents area does not offer it", op)
		}
	}
	for _, op := range ngac.MemberChannelOps() {
		if !slices.Contains(ngac.AreaOps(ngac.AreaChannels), op) {
			t.Errorf("members hold %q on Channels but the channels area does not offer it", op)
		}
	}
}

func TestUnknownAreaOffersNothing(t *testing.T) {
	if ngac.IsArea("nope") || ngac.AreaOps("nope") != nil {
		t.Fatal("an unknown area must offer nothing")
	}
	if _, ok := ngac.AreaOAName("nope", "ws"); ok {
		t.Fatal("an unknown area has no OA")
	}
}

func TestAreaOpsReturnsACopy(t *testing.T) {
	ops := ngac.AreaOps(ngac.AreaDocuments)
	ops[0] = "tampered"
	if ngac.AreaOps(ngac.AreaDocuments)[0] == "tampered" {
		t.Fatal("AreaOps must not share its slice")
	}
}

func TestAreaOANames(t *testing.T) {
	want := map[ngac.Area]string{
		ngac.AreaManagement: ngac.MgmtOAName("w"),
		ngac.AreaDocuments:  ngac.DocumentsOAName("w"),
		ngac.AreaChannels:   ngac.ChannelsOAName("w"),
		ngac.AreaAssets:     ngac.AssetsOAName("w"),
	}
	for a, name := range want {
		got, ok := ngac.AreaOAName(a, "w")
		if !ok || got != name {
			t.Errorf("area %q: got %q, want %q", a, got, name)
		}
	}
}

// Every operation an area offers is checked by some service on that kind of OA.
// `upload` is granted to members but never checked (the drive gates an upload on
// write), so no area may offer it.
func TestNoAreaOffersAnOperationNoServiceChecks(t *testing.T) {
	for _, a := range ngac.Areas() {
		if slices.Contains(ngac.AreaOps(a), ngac.OpUpload) {
			t.Errorf("area %q offers upload, which no service checks", a)
		}
	}
}
