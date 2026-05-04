package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/policy"
)

// TestPCGuard_NoScope_Rejected verifies that creating a PC without scope is rejected.
func TestPCGuard_NoScope_Rejected(t *testing.T) {
	ws := &WriteServer{} // nil store is fine — guard fires before DB call

	_, err := ws.CreateNode(context.Background(), &pb.CreateNodeRequest{
		Name:     "PC_Rogue",
		NodeType: "PC",
		Properties: map[string]string{
			"workspace": "test",
		},
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
	assert.Contains(t, st.Message(), "scope")
}

// TestPCGuard_TenantScopeWithoutTenantID_Rejected verifies that non-global scope requires tenant_id.
func TestPCGuard_TenantScopeWithoutTenantID_Rejected(t *testing.T) {
	ws := &WriteServer{}

	_, err := ws.CreateNode(context.Background(), &pb.CreateNodeRequest{
		Name:     "PC_BadTenant",
		NodeType: "PC",
		Properties: map[string]string{
			"scope": "tenant",
			// missing tenant_id
		},
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
	assert.Contains(t, st.Message(), "tenant_id")
}

// TestPCGuard_GlobalScope_NoTenantRequired verifies that global scope doesn't require tenant_id.
// The guard passes, but nil store panics — we recover and confirm the guard didn't fire.
func TestPCGuard_GlobalScope_NoTenantRequired(t *testing.T) {
	ws := &WriteServer{}

	var guardPassed bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Panic from nil store means guard passed
				guardPassed = true
			}
		}()
		_, err := ws.CreateNode(context.Background(), &pb.CreateNodeRequest{
			Name:     "PC_Global",
			NodeType: "PC",
			Properties: map[string]string{
				"scope": "global",
			},
		})
		if err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() != codes.PermissionDenied {
				guardPassed = true
			}
		}
	}()
	assert.True(t, guardPassed, "global scope should pass the PC guard")
}

// TestPCGuard_NonPCNode_NoGuard verifies that non-PC nodes bypass the guard entirely.
func TestPCGuard_NonPCNode_NoGuard(t *testing.T) {
	ws := &WriteServer{}

	for _, nodeType := range []string{"U", "UA", "OA", "O"} {
		t.Run(nodeType, func(t *testing.T) {
			var guardPassed bool
			func() {
				defer func() {
					if r := recover(); r != nil {
						guardPassed = true // panic from nil store = guard passed
					}
				}()
				_, err := ws.CreateNode(context.Background(), &pb.CreateNodeRequest{
					Name:       "TestNode",
					NodeType:   nodeType,
					Properties: map[string]string{}, // no scope — fine for non-PC
				})
				if err != nil {
					st, _ := status.FromError(err)
					if st.Code() != codes.PermissionDenied {
						guardPassed = true
					}
				}
			}()
			assert.True(t, guardPassed,
				"node type %s should NOT trigger PC guard", nodeType)
		})
	}
}
