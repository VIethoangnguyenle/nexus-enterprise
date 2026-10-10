package grpc

import (
	"ngac-platform/pkg/grpcutil"

	"fmt"
)

func domainError(context string, err error) error {
	if err == nil {
		return nil
	}
	return grpcutil.Status(fmt.Errorf("%s: %w", context, err))
}
