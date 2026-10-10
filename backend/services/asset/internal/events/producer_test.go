package events

import (
	"context"
	"encoding/json"
	"testing"

	"ngac-platform/pkg/grpcauth"
)

type sent struct {
	topic, key string
	data       []byte
}

func capturing() (*Producer, *[]sent) {
	var out []sent
	return &Producer{send: func(topic, key string, data []byte) { out = append(out, sent{topic, key, data}) }}, &out
}

func callerCtx(tenant string) context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u1", NGACNodeID: "n1", TenantID: tenant})
}

func TestEveryAssetEventCarriesTheCallersTenant(t *testing.T) {
	p, out := capturing()
	ctx := callerCtx("tenant-a")

	p.PublishLifecycle(ctx, LifecycleEvent{AssetID: "a1", WorkspaceID: "w1", TenantID: "forged"})
	p.PublishRequest(ctx, RequestEvent{RequestID: "r1", WorkspaceID: "w1"})
	p.PublishAssignment(ctx, AssignmentEvent{AssetID: "a1", WorkspaceID: "w1"})

	if len(*out) != 3 {
		t.Fatalf("sent %d events, want 3", len(*out))
	}
	wantTopics := []string{TopicLifecycle, TopicRequest, TopicAssignment}
	for i, s := range *out {
		var got struct {
			TenantID    string `json:"tenant_id"`
			WorkspaceID string `json:"workspace_id"`
		}
		if err := json.Unmarshal(s.data, &got); err != nil {
			t.Fatal(err)
		}
		if s.topic != wantTopics[i] {
			t.Errorf("topic = %q, want %q", s.topic, wantTopics[i])
		}
		if got.TenantID != "tenant-a" || got.WorkspaceID != "w1" {
			t.Errorf("%s: tenant_id = %q workspace_id = %q, want tenant-a / w1 (the caller's tenant, never the body's)", s.topic, got.TenantID, got.WorkspaceID)
		}
	}
}

func TestNilProducerDiscards(t *testing.T) {
	var p *Producer
	p.PublishLifecycle(callerCtx("t"), LifecycleEvent{})
	p.PublishRequest(callerCtx("t"), RequestEvent{})
	p.PublishAssignment(callerCtx("t"), AssignmentEvent{})
}

// A workspace's events are keyed by workspace, so they keep their order on one
// partition whichever asset or request they name.
func TestPublishKeysByWorkspace(t *testing.T) {
	var keys []string
	p := &Producer{send: func(_, key string, _ []byte) { keys = append(keys, key) }}
	ctx := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u", NGACNodeID: "n", TenantID: "t"})
	p.PublishLifecycle(ctx, LifecycleEvent{AssetID: "a1", WorkspaceID: "w1"})
	p.PublishRequest(ctx, RequestEvent{RequestID: "r1", WorkspaceID: "w1"})
	p.PublishAssignment(ctx, AssignmentEvent{AssetID: "a2", WorkspaceID: "w1"})
	if len(keys) != 3 || keys[0] != "w1" || keys[1] != "w1" || keys[2] != "w1" {
		t.Fatalf("keys = %v", keys)
	}
}
