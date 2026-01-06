package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/infraerror"
)

func recordInfrastructureError(ctx context.Context, component, operation string, err error) {
	infraerror.RecordInfrastructureError(ctx, component, operation, err)
}

func recordDatabaseError(ctx context.Context, operation string, err error) {
	infraerror.RecordInfrastructureError(ctx, "db", operation, err)
}

func recordRedisError(ctx context.Context, operation string, err error) {
	infraerror.RecordInfrastructureError(ctx, "redis", operation, err)
}
