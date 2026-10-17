package main

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	pb "ngac-platform/proto/policy"
)

type fakeInvalidator struct {
	batches [][]string
	reasons []string
	failAt  int // 1-based call that fails; 0 never
}

func (f *fakeInvalidator) InvalidateCache(_ context.Context, in *pb.InvalidateCacheRequest, _ ...grpc.CallOption) (*pb.InvalidateCacheResponse, error) {
	f.batches = append(f.batches, in.GetNodeIds())
	f.reasons = append(f.reasons, in.GetReason())
	if f.failAt == len(f.batches) {
		return nil, errors.New("writer unavailable")
	}
	return &pb.InvalidateCacheResponse{}, nil
}

func TestInvalidateInBatches(t *testing.T) {
	f := &fakeInvalidator{}
	ids := []string{"a", "b", "c", "d", "e"}
	if err := invalidateInBatches(context.Background(), f, ids, 2, "migration"); err != nil {
		t.Fatal(err)
	}
	if len(f.batches) != 3 || len(f.batches[0]) != 2 || len(f.batches[2]) != 1 {
		t.Fatalf("batches = %v, want sizes 2,2,1", f.batches)
	}
	if f.reasons[0] != "migration" {
		t.Fatalf("reason = %q", f.reasons[0])
	}
}

func TestInvalidateInBatchesStopsOnFailure(t *testing.T) {
	f := &fakeInvalidator{failAt: 2}
	err := invalidateInBatches(context.Background(), f, []string{"a", "b", "c", "d", "e"}, 2, "r")
	if err == nil {
		t.Fatal("a failed batch must fail the run: the stale decisions are still being served")
	}
	if len(f.batches) != 2 {
		t.Fatalf("made %d calls, want to stop at the failing one (2)", len(f.batches))
	}
}

func TestInvalidateInBatchesWithNothingToDo(t *testing.T) {
	f := &fakeInvalidator{}
	if err := invalidateInBatches(context.Background(), f, nil, 10, "r"); err != nil {
		t.Fatal(err)
	}
	if len(f.batches) != 0 {
		t.Fatalf("called the writer with no node ids: %v", f.batches)
	}
}

func TestParseInvalidateArgs(t *testing.T) {
	if _, err := parseInvalidateArgs(nil); err == nil {
		t.Fatal("without --all-workspaces there is nothing to invalidate; want a usage error")
	}
	if _, err := parseInvalidateArgs([]string{"--all-workspaces", "--batch", "0"}); err == nil {
		t.Fatal("batch 0 would never advance; want an error")
	}
	o, err := parseInvalidateArgs([]string{"--all-workspaces", "--batch", "7"})
	if err != nil || o.batch != 7 {
		t.Fatalf("= (%+v, %v)", o, err)
	}
}
