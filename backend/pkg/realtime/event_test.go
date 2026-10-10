package realtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"ngac-platform/pkg/grpcauth"
)

func valid() Event {
	return Event{Domain: DomainDrive, Kind: KindCreated, TenantID: "t1", WorkspaceID: "w1", IDs: []string{"a"}}
}

func TestValidateRequiresTenantAndWorkspace(t *testing.T) {
	cases := map[string]func(*Event){
		"no domain":    func(e *Event) { e.Domain = "" },
		"no kind":      func(e *Event) { e.Kind = "" },
		"no tenant":    func(e *Event) { e.TenantID = "" },
		"no workspace": func(e *Event) { e.WorkspaceID = "" },
		"colon tenant": func(e *Event) { e.TenantID = "a:b" },
		"colon node":   func(e *Event) { e.UserNodeIDs = []string{"x:y"} },
	}
	for name, mutate := range cases {
		e := valid()
		mutate(&e)
		if err := e.Validate(); err == nil {
			t.Errorf("%s: want error, got nil", name)
		}
	}
	if err := valid().Validate(); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
}

func TestLevelFollowsAddressing(t *testing.T) {
	e := valid()
	if e.Level() != LevelWorkspace {
		t.Fatal("unaddressed event must be workspace-wide")
	}
	e.ChannelID = "c1"
	if e.Level() != LevelChannel {
		t.Fatal("channel id selects channel level")
	}
	e.UserNodeIDs = []string{"u1"}
	if e.Level() != LevelUser {
		t.Fatal("user nodes take precedence over channel")
	}
}

func TestTopicAndDecode(t *testing.T) {
	if Topic(DomainDrive) != "drive.events" {
		t.Fatalf("topic = %q", Topic(DomainDrive))
	}
	if _, err := Decode([]byte(`{"domain":"drive"`)); err == nil {
		t.Fatal("malformed json must fail")
	}
	if _, err := Decode([]byte(`{"domain":"drive","kind":"created"}`)); err == nil || !strings.Contains(err.Error(), "tenant") {
		t.Fatalf("event without tenant must fail, got %v", err)
	}
	e, err := Decode([]byte(`{"domain":"drive","kind":"created","tenant_id":"t","workspace_id":"w","ids":["a"]}`))
	if err != nil || e.IDs[0] != "a" {
		t.Fatalf("decode: %+v %v", e, err)
	}
}

func TestNilProducerAndEmitterAreSafe(t *testing.T) {
	var p *Producer
	p.Emit(valid())
	p.Close()
}

func TestForTakesTenantAndActorFromTheCaller(t *testing.T) {
	ctx := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u1", NGACNodeID: "n1", TenantID: "t1"})
	e := For(ctx, DomainDrive, KindDeleted, "w1", "a", "b")
	if e.TenantID != "t1" || e.ActorUserID != "u1" || e.WorkspaceID != "w1" || len(e.IDs) != 2 {
		t.Fatalf("unexpected %+v", e)
	}
	if err := For(context.Background(), DomainDrive, KindDeleted, "w1").Validate(); err == nil {
		t.Fatal("a request with no caller has no tenant and must not validate")
	}
}

// With the broker unreachable, announcing a change must not slow the request
// that made it, however many are made.
func TestEmitDoesNotBlockWhenTheBrokerIsDown(t *testing.T) {
	p, err := NewProducer([]string{"127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	start := time.Now()
	for i := 0; i < 3*maxBufferedRecords; i++ {
		p.Emit(valid())
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("emit blocked for %v with the broker down", d)
	}
}
