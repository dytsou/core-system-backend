package tenant

import (
	"NYCU-SDC/core-system-backend/internal"
	"context"

	databaseutil "github.com/NYCU-SDC/summer/pkg/database"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Querier interface {
	Create(ctx context.Context, param CreateParams) (Tenant, error)
	Get(ctx context.Context, id uuid.UUID) (Tenant, error)
	Update(ctx context.Context, param UpdateParams) (Tenant, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	logger *zap.Logger
	tracer trace.Tracer
	query  Querier
}

func NewService(logger *zap.Logger, db DBTX) *Service {
	return &Service{
		logger: logger,
		tracer: otel.Tracer("tenant/service"),
		query:  New(db),
	}
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (Tenant, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetByID")
	defer span.End()
	logger := internal.WithContext(traceCtx, s.logger)

	tenant, err := s.query.Get(traceCtx, id)
	if err != nil {
		err = databaseutil.WrapDBErrorWithKeyValue(err, "tenants", "id", id.String(), logger, "get tenant by id")
		span.RecordError(err)
		return Tenant{}, err
	}

	return tenant, nil
}

func (s *Service) CreateByID(ctx context.Context, id uuid.UUID) (Tenant, error) {
	traceCtx, span := s.tracer.Start(ctx, "CreateByID")
	defer span.End()
	logger := internal.WithContext(traceCtx, s.logger)

	tenant, err := s.query.Create(traceCtx, CreateParams{
		ID:         id,
		DbStrategy: DbStrategyShared,
	})
	if err != nil {
		err = databaseutil.WrapDBErrorWithKeyValue(err, "tenants", "id", id.String(), logger, "create tenant by id")
		span.RecordError(err)
		return Tenant{}, err
	}

	logger.Info("tenant created", zap.String("tenant_id", tenant.ID.String()), zap.String("db_strategy", string(tenant.DbStrategy)))

	return tenant, nil
}

func (s *Service) UpdateByID(ctx context.Context, param UpdateParams) (Tenant, error) {
	traceCtx, span := s.tracer.Start(ctx, "UpdateByID")
	defer span.End()
	logger := internal.WithContext(traceCtx, s.logger)

	tenant, err := s.query.Update(traceCtx, param)
	if err != nil {
		err = databaseutil.WrapDBErrorWithKeyValue(err, "tenants", "id", param.ID.String(), logger, "update tenant by id")
		span.RecordError(err)
		return Tenant{}, err
	}

	logger.Info("tenant updated", zap.String("tenant_id", tenant.ID.String()), zap.String("db_strategy", string(tenant.DbStrategy)))

	return tenant, nil
}

func (s *Service) DeleteByID(ctx context.Context, id uuid.UUID) error {
	traceCtx, span := s.tracer.Start(ctx, "DeleteByID")
	defer span.End()
	logger := internal.WithContext(traceCtx, s.logger)

	err := s.query.Delete(traceCtx, id)
	if err != nil {
		err = databaseutil.WrapDBErrorWithKeyValue(err, "tenants", "id", id.String(), logger, "delete tenant by id")
		span.RecordError(err)
		return err
	}
	logger.Info("tenant deleted", zap.String("tenant_id", id.String()))
	return nil
}
