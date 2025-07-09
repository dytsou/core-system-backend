package internal

import (
	"context"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"go.uber.org/zap"
)

// WithContext parses the context and adds the organization ID and slug to the logger if available
func WithContext(ctx context.Context, logger *zap.Logger) *zap.Logger {
	logger = logutil.WithContext(ctx, logger)
	if ctx == nil {
		return logger
	}

	orgID, ok := ctx.Value(OrgIDContextKey).(string)
	if ok && orgID != "" {
		logger = logger.With(zap.String("org_id", orgID))
	}

	orgSlug, ok := ctx.Value(OrgSlugContextKey).(string)
	if ok && orgSlug != "" {
		logger = logger.With(zap.String("org_slug", orgSlug))
	}

	return logger
}
