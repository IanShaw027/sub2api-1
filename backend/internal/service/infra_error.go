package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/infraerror"
)

func recordInfrastructureError(ctx context.Context, component, operation string, err error) {
	infraerror.RecordInfrastructureError(ctx, component, operation, err)
}

