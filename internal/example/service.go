package example

import (
	"context"
	databaseutil "github.com/NYCU-SDC/summer/pkg/database"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

//go:generate mockery --name Querier
type Querier interface {
	GetAll(ctx context.Context) ([]Scoreboard, error)
	GetByID(ctx context.Context, id uuid.UUID) (Scoreboard, error)
	Create(ctx context.Context, name string) (Scoreboard, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Update(ctx context.Context, arg UpdateParams) (Scoreboard, error)
}

type Service struct {
	logger  *zap.Logger
	tracer  trace.Tracer
	queries *Queries
}

func NewService(logger *zap.Logger, db *pgxpool.Pool) *Service {
	return &Service{
		logger:  logger,
		tracer:  otel.Tracer("example/service"),
		queries: New(db),
	}
}

func (s Service) GetAll(ctx context.Context) ([]Scoreboard, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetAll")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	scoreboards, err := s.queries.GetAll(ctx)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "failed to get all scoreboards")
		span.RecordError(err)
		return nil, err
	}

	return scoreboards, nil
}

func (s Service) GetByID(ctx context.Context, id uuid.UUID) (Scoreboard, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetByID")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	scoreboard, err := s.queries.GetByID(ctx, id)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "failed to get scoreboard by id")
		span.RecordError(err)
		return Scoreboard{}, err
	}

	return scoreboard, nil
}

func (s Service) Create(ctx context.Context, req CreateRequest) (Scoreboard, error) {
	traceCtx, span := s.tracer.Start(ctx, "Create")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	scoreboard, err := s.queries.Create(ctx, req.Name)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "failed to create scoreboard")
		span.RecordError(err)
		return Scoreboard{}, err
	}

	return scoreboard, nil
}

func (s Service) Update(ctx context.Context, id uuid.UUID, r UpdateRequest) (Scoreboard, error) {
	traceCtx, span := s.tracer.Start(ctx, "Update")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	scoreboard, err := s.queries.Update(ctx, UpdateParams{
		ID:   id,
		Name: r.Name,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "failed to update scoreboard")
		span.RecordError(err)
		return Scoreboard{}, err
	}

	return scoreboard, nil
}

func (s Service) Delete(ctx context.Context, id uuid.UUID) error {
	traceCtx, span := s.tracer.Start(ctx, "Delete")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	err := s.queries.Delete(ctx, id)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "failed to delete scoreboard")
		span.RecordError(err)
		return err
	}

	return nil
}
