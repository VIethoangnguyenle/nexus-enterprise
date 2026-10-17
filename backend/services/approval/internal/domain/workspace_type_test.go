package domain

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"ngac-platform/pkg/grpcauth"
)

// A personal workspace has no approvals. Every entry point of the service
// refuses there, before it reads or writes anything: the store below answers
// only the workspace's type, and any other call panics through the nil
// embedded interface.

type typeOnlyStore struct {
	Store
	kind  string
	err   error
	asked []string
}

func (s *typeOnlyStore) WorkspaceType(_ context.Context, id string) (string, error) {
	s.asked = append(s.asked, id)
	return s.kind, s.err
}

func callerIn(tenant string) context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u", NGACNodeID: "n-u", TenantID: tenant})
}

// entryPoint calls one exported method of Service with arguments that would
// succeed past the guard. tenantArg is true where the method is handed the
// tenant it acts on instead of reading it from the caller.
type entryPoint struct {
	name      string
	tenantArg bool
	call      func(s *Service, ctx context.Context) error
}

func entryPoints() []entryPoint {
	return []entryPoint{
		{"CreateApprovalRequest", false, func(s *Service, ctx context.Context) error {
			_, err := s.CreateApprovalRequest(ctx, CreateRequestInput{EntityType: "expense", CreatedBy: "n-u"})
			return err
		}},
		{"Approve", false, func(s *Service, ctx context.Context) error {
			return s.Approve(ctx, ApproveInput{RequestID: "r", UserNodeID: "n-u"})
		}},
		{"Reject", false, func(s *Service, ctx context.Context) error {
			return s.Reject(ctx, RejectInput{RequestID: "r", UserNodeID: "n-u"})
		}},
		{"BatchApprove", false, func(s *Service, ctx context.Context) error {
			_, err := s.BatchApprove(ctx, BatchApproveInput{RequestIDs: []string{"r"}, UserNodeID: "n-u"})
			return err
		}},
		{"ResolveTemplate", false, func(s *Service, ctx context.Context) error {
			_, err := s.ResolveTemplate(ctx, "expense", EntityFields{})
			return err
		}},
		{"ProvisionTenantSchema", true, func(s *Service, ctx context.Context) error {
			_, err := s.ProvisionTenantSchema(ctx, "tenant-1")
			return err
		}},
		{"EnsureTenantSchema", true, func(s *Service, ctx context.Context) error {
			_, err := s.EnsureTenantSchema(ctx, "tenant-1")
			return err
		}},
		{"GetPending", false, func(s *Service, ctx context.Context) error {
			_, err := s.GetPending(ctx, "n-u")
			return err
		}},
		{"GetHistory", false, func(s *Service, ctx context.Context) error {
			_, _, err := s.GetHistory(ctx, "n-u", "", 10)
			return err
		}},
		{"GetMyRequests", false, func(s *Service, ctx context.Context) error {
			_, _, err := s.GetMyRequests(ctx, "n-u", "", 10)
			return err
		}},
		{"GetDepartmentRequests", false, func(s *Service, ctx context.Context) error {
			_, _, err := s.GetDepartmentRequests(ctx, "n-u", "", 10)
			return err
		}},
		{"GetAuditLog", false, func(s *Service, ctx context.Context) error {
			_, err := s.GetAuditLog(ctx, "n-u", "11111111-1111-4111-8111-111111111111")
			return err
		}},
		{"GetRequestDetail", false, func(s *Service, ctx context.Context) error {
			_, err := s.GetRequestDetail(ctx, "n-u", "11111111-1111-4111-8111-111111111111")
			return err
		}},
		{"CanManageTemplates", true, func(s *Service, ctx context.Context) error {
			_, err := s.CanManageTemplates(ctx, "n-u", "tenant-1")
			return err
		}},
		{"CreateTemplate", false, func(s *Service, ctx context.Context) error {
			_, err := s.CreateTemplate(ctx, "n-u", CreateTemplateInput{TenantID: "tenant-1", Name: "T", EntityType: "expense"})
			return err
		}},
		{"GetTemplate", false, func(s *Service, ctx context.Context) error {
			_, err := s.GetTemplate(ctx, "t")
			return err
		}},
		{"ListTemplates", false, func(s *Service, ctx context.Context) error {
			_, err := s.ListTemplates(ctx, "expense", true)
			return err
		}},
		{"UpdateTemplate", false, func(s *Service, ctx context.Context) error {
			_, err := s.UpdateTemplate(ctx, "n-u", "t", UpdateTemplateInput{TenantID: "tenant-1", ExpectedUpdatedAt: time.Now()})
			return err
		}},
	}
}

func TestPersonalWorkspace_EveryEntryPointIsRefused(t *testing.T) {
	for _, ep := range entryPoints() {
		t.Run(ep.name, func(t *testing.T) {
			st := &typeOnlyStore{kind: "personal"}

			err := ep.call(NewService(st, nil), callerIn("tenant-1"))

			if !errors.Is(err, ErrAccessDenied) {
				t.Fatalf("error = %v; want ErrAccessDenied", err)
			}
			if !strings.Contains(err.Error(), "personal workspace") || strings.Contains(err.Error(), "tenant-1") {
				t.Errorf("message %q should name the reason and no ID", err)
			}
			if len(st.asked) == 0 || st.asked[0] != "tenant-1" {
				t.Errorf("workspace type asked for %v; want the caller's tenant first", st.asked)
			}
		})
	}
}

// A method handed a tenant of its own, and one that reads the caller's, must
// both be organizations: signing in to an organization does not make another
// tenant's id acceptable.
func TestTemplateWrites_BothTenantsMustBeOrganizations(t *testing.T) {
	kinds := map[string]string{"signed-in": "organization", "named": "personal"}
	st := &mapStore{kinds: kinds}
	s := NewService(st, nil)

	_, err := s.CreateTemplate(callerIn("signed-in"), "n-u", CreateTemplateInput{TenantID: "named", Name: "T", EntityType: "expense"})

	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("CreateTemplate for a personal tenant while signed in to an organization: %v", err)
	}
}

type mapStore struct {
	Store
	kinds map[string]string
}

func (m *mapStore) WorkspaceType(_ context.Context, id string) (string, error) {
	k, ok := m.kinds[id]
	if !ok {
		return "", ErrNotFound
	}
	return k, nil
}

func TestGuard_FailsClosed(t *testing.T) {
	boom := errors.New("connection reset by peer")
	cases := []struct {
		name    string
		store   *typeOnlyStore
		denied  bool // ErrAccessDenied; otherwise an unclassified failure
		allowed bool
	}{
		{"organization is allowed", &typeOnlyStore{kind: "organization"}, false, true},
		{"unknown workspace is refused", &typeOnlyStore{err: ErrNotFound}, true, false},
		{"a type that is neither is refused", &typeOnlyStore{kind: "something-new"}, true, false},
		{"an empty type is refused", &typeOnlyStore{kind: ""}, true, false},
		{"a failed read is an error, not an allow", &typeOnlyStore{err: boom}, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := NewService(c.store, nil).RequireOrganization(context.Background(), "tenant-1")

			switch {
			case c.allowed && err != nil:
				t.Fatalf("refused: %v", err)
			case c.allowed:
			case err == nil:
				t.Fatal("allowed")
			case c.denied && !errors.Is(err, ErrAccessDenied):
				t.Errorf("error = %v; want ErrAccessDenied", err)
			case !c.denied && (errors.Is(err, ErrAccessDenied) || !errors.Is(err, boom)):
				t.Errorf("error = %v; want the unclassified failure wrapping the cause", err)
			}
		})
	}
}

// A new exported method without a row in entryPoints is an entry point nobody
// proved refuses in a personal workspace.
func TestEntryPointTableCoversEveryExportedMethod(t *testing.T) {
	// Guard and event helpers that are not entry points: RequireOrganization and
	// RequireMember are the guards; EventAudience runs only after a decision that
	// was guarded.
	notEntryPoints := map[string]bool{"RequireOrganization": true, "RequireMember": true, "EventAudience": true}
	var want, got []string
	typ := reflect.TypeOf(&Service{})
	for i := 0; i < typ.NumMethod(); i++ {
		if n := typ.Method(i).Name; !notEntryPoints[n] {
			want = append(want, n)
		}
	}
	for _, ep := range entryPoints() {
		got = append(got, ep.name)
	}
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("entryPoints() = %v\nexported methods = %v", got, want)
	}
}

// BatchApprove skips what it cannot approve; in a personal workspace that must
// not turn into an empty 200.
func TestBatchApprove_PersonalWorkspaceIsAnErrorNotAnEmptyList(t *testing.T) {
	st := &typeOnlyStore{kind: "personal"}

	approved, err := NewService(st, nil).BatchApprove(callerIn("tenant-1"), BatchApproveInput{RequestIDs: []string{"a", "b"}, UserNodeID: "n-u"})

	if !errors.Is(err, ErrAccessDenied) || len(approved) != 0 {
		t.Fatalf("approved %v, error %v", approved, err)
	}
	if len(st.asked) != 1 {
		t.Errorf("workspace type read %d times; want once", len(st.asked))
	}
}
