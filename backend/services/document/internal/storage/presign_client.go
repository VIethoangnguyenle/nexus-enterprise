package storage

import (
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// presignRegion is fixed so the signing client never asks the store for the
// bucket's region: it only computes a signature and must make no network call
// (the public host is not necessarily reachable from this service).
const presignRegion = "us-east-1"

// PublicEndpoint is where browsers reach the object store.
type PublicEndpoint struct {
	// Host is host[:port] with no scheme or path; it is part of the signature, so
	// the proxy in front of the store must forward it unchanged.
	Host string
	// Secure makes signed URLs https.
	Secure bool
}

// NewPresignClient returns a client that signs URLs for the public endpoint. It
// is separate from the client used for server-side operations, which talks to
// the store on the internal network.
func NewPresignClient(pub PublicEndpoint, accessKey, secretKey string) (*minio.Client, error) {
	c, err := minio.New(pub.Host, &minio.Options{
		Creds:        credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:       pub.Secure,
		Region:       presignRegion,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("presign client for %q: %w", pub.Host, err)
	}
	return c, nil
}
