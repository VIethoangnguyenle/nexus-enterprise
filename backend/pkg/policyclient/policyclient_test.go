package policyclient_test

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	policypb "ngac-platform/proto/policy"
)

type fakePolicy struct {
	decision string
	err      error
	batch    *policypb.BatchAccessResult
	calls    int
	lastReq  *policypb.CheckAccessRequest
}

func (f *fakePolicy) CheckAccess(_ context.Context, in *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	f.calls++
	f.lastReq = in
	if f.err != nil {
		return nil, f.err
	}
	return &policypb.AccessDecision{Decision: f.decision}, nil
}

func (f *fakePolicy) BatchCheckAccess(_ context.Context, _ *policypb.BatchCheckAccessRequest, _ ...grpc.CallOption) (*policypb.BatchAccessResult, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.batch, nil
}

func TestCheckAllowsOnlyAnExplicitAllow(t *testing.T) {
	cases := []struct {
		name     string
		decision string
		err      error
		want     bool
		wantErr  bool
	}{
		{"allow", ngac.DecisionAllow, nil, true, false},
		{"deny", ngac.DecisionDeny, nil, false, false},
		{"unrecognised decision", "MAYBE", nil, false, false},
		{"empty decision", "", nil, false, false},
		{"outage is never an allow", ngac.DecisionAllow, errors.New("unavailable"), false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := policyclient.New(&fakePolicy{decision: tc.decision, err: tc.err})
			got, err := c.Check(context.Background(), "u", "oa", ngac.OpRead)
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("Check = (%v, %v), want (%v, err=%v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

// Identity is the PDP's to judge: an empty user or object is sent as it is, and
// whatever the PDP answers is what the client reports.
func TestCheckForwardsEmptyIdentitiesToThePDP(t *testing.T) {
	f := &fakePolicy{decision: ngac.DecisionDeny}
	c := policyclient.New(f)
	for _, tc := range [][2]string{{"", "oa"}, {"u", ""}, {"", ""}} {
		if ok, err := c.Check(context.Background(), tc[0], tc[1], ngac.OpRead); ok || err != nil {
			t.Errorf("Check(%q,%q) = (%v, %v), want (false, nil)", tc[0], tc[1], ok, err)
		}
	}
	if f.calls != 3 {
		t.Errorf("policy service was asked %d times, want 3", f.calls)
	}
}

func TestCheckCallerUsesTheVerifiedCaller(t *testing.T) {
	f := &fakePolicy{decision: ngac.DecisionAllow}
	c := policyclient.New(f)

	ctx := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u1", NGACNodeID: "node-1"})
	ok, err := c.CheckCaller(ctx, "oa-1", ngac.OpWrite)
	if !ok || err != nil {
		t.Fatalf("CheckCaller = (%v, %v)", ok, err)
	}
	if f.lastReq.GetUserNodeId() != "node-1" || f.lastReq.GetObjectNodeId() != "oa-1" || f.lastReq.GetOperation() != ngac.OpWrite {
		t.Errorf("asked %+v", f.lastReq)
	}

	denying := &fakePolicy{decision: ngac.DecisionDeny}
	if ok, _ := policyclient.New(denying).CheckCaller(context.Background(), "oa-1", ngac.OpWrite); ok {
		t.Error("no caller on the context asks the PDP about nobody, which denies")
	}
	if denying.lastReq.GetUserNodeId() != "" {
		t.Errorf("asked about %q", denying.lastReq.GetUserNodeId())
	}
}

func TestBatchCheck(t *testing.T) {
	f := &fakePolicy{batch: &policypb.BatchAccessResult{Results: map[string]*policypb.ObjectPermissions{
		"oa-1": {Permissions: map[string]bool{ngac.OpRead: true, ngac.OpWrite: false}},
	}}}
	c := policyclient.New(f)

	perms, err := c.BatchCheck(context.Background(), "u", []string{"oa-1", "oa-2"}, []string{ngac.OpRead, ngac.OpWrite})
	if err != nil {
		t.Fatal(err)
	}
	if !perms.Has("oa-1", ngac.OpRead) {
		t.Error("oa-1 read should be held")
	}
	if perms.Has("oa-1", ngac.OpWrite) || perms.Has("oa-2", ngac.OpRead) || perms.Has("missing", ngac.OpRead) {
		t.Error("anything not granted must read as not held")
	}
}

func TestBatchCheckFailsClosed(t *testing.T) {
	c := policyclient.New(&fakePolicy{err: errors.New("down")})
	perms, err := c.BatchCheck(context.Background(), "u", []string{"oa"}, []string{ngac.OpRead})
	if err == nil {
		t.Fatal("an outage must be reported")
	}
	if perms.Has("oa", ngac.OpRead) {
		t.Error("a nil Permissions must hold nothing")
	}

	f := &fakePolicy{}
	empty := policyclient.New(f)
	for _, tc := range []struct {
		user string
		objs []string
		ops  []string
	}{{"u", nil, []string{"read"}}, {"u", []string{"oa"}, nil}} {
		p, err := empty.BatchCheck(context.Background(), tc.user, tc.objs, tc.ops)
		if err != nil || p.Has("oa", "read") {
			t.Errorf("empty input: (%v, %v)", p, err)
		}
	}
	if f.calls != 0 {
		t.Errorf("nothing to ask must not reach the policy service, calls=%d", f.calls)
	}
}

func TestIsNotFoundSeparatesAbsentFromUnreachable(t *testing.T) {
	if !policyclient.IsNotFound(status.Error(codes.NotFound, "no node")) {
		t.Error("NotFound is absent")
	}
	for _, err := range []error{nil, errors.New("down"), status.Error(codes.Unavailable, "down"), status.Error(codes.PermissionDenied, "no")} {
		if policyclient.IsNotFound(err) {
			t.Errorf("%v is not 'absent'", err)
		}
	}
}
