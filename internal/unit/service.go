package unit

import (
	"NYCU-SDC/core-system-backend/internal"
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Querier interface {
}

type Service struct {
	logger *zap.Logger
	tracer trace.Tracer
}

func NewService(logger *zap.Logger) *Service {
	return &Service{
		logger: logger,
		tracer: otel.Tracer("unit/service"),
	}
}

func (s *Service) CreateOrg(ctx context.Context, name, description, slug string) (Organization, error) {
	traceCtx, span := s.tracer.Start(ctx, "CreateOrg")
	defer span.End()
	logger := internal.WithContext(traceCtx, s.logger)

	conn, err := internal.GetDBTXFromContext(traceCtx)
	if err != nil {
		err = internal.WrapDBError(err, logger, "get DBTX from context")
		span.RecordError(err)
		return Organization{}, err
	}

	querier := New(conn)

	org, err := s.queries.CreateOrg(traceCtx, CreateOrgParams{
		Name:        name,
		Description: description,
		Slug:        slug,
	})
}
