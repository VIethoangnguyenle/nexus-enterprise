package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/policy"
	"ngac-platform/services/policy/internal/ngac"
)

// A write the database cannot take is a failure of ours: the caller gets a
// generic Internal, and the SQL text (table, constraint, driver) stays in the
// server's log, where the logging interceptor writes the error it sees.
func TestWriteServer_DatabaseFailureIsAGenericInternal(t *testing.T) {
	store, pool := setupWriteTestStore(t)
	pool.Close() // every statement now fails with the driver's own message

	ws := NewWriteServer(store, nil, ngac.NewInvalidationCoordinator(nil), nil,
		ngac.NewProhibitionStore(pool, store.GetGraph()), false)

	_, err := ws.CreateNode(context.Background(), &pb.CreateNodeRequest{Name: "leak-check-" + t.Name(), NodeType: "UA"})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Equal(t, "internal error", status.Convert(err).Message())
	assert.NotEmpty(t, err.Error(), "the in-process error keeps its text for the log")
}
