package inbox

import (
	"context"

	databaseutil "github.com/NYCU-SDC/summer/pkg/database"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Querier interface {
	ListAllByUserID(ctx context.Context, userID uuid.UUID) ([]ListAllByUserIDRow, error)
	GetById(ctx context.Context, arg GetByIdParams) (GetByIdRow, error)
	UpdateById(ctx context.Context, arg UpdateByIdParams) (UpdateByIdRow, error)
}

type Service struct {
	logger  *zap.Logger
	queries Querier
	tracer  trace.Tracer
}

func NewService(logger *zap.Logger, db DBTX) *Service {
	return &Service{
		logger:  logger,
		queries: New(db),
		tracer:  otel.Tracer("inbox/service"),
	}
}

func (s *Service) GetAll(ctx context.Context, userId uuid.UUID) ([]ListAllByUserIDRow, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetAll")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	messages, err := s.queries.ListAllByUserID(traceCtx, userId)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "get all user inbox messages")
		span.RecordError(err)
		return nil, err
	}

	if messages == nil {
		return []ListAllByUserIDRow{}, err
	}

	return messages, err
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID, userId uuid.UUID) (GetByIdRow, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetByID")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	message, err := s.queries.GetById(traceCtx, GetByIdParams{
		ID:     id,
		UserID: userId,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "get the full inbox_message by id")
		span.RecordError(err)
		return GetByIdRow{}, err
	}

	return message, err
}

func (s *Service) UpdateByID(ctx context.Context, id uuid.UUID, userId uuid.UUID, arg UserInboxMessageFilter) (UpdateByIdRow, error) {
	traceCtx, span := s.tracer.Start(ctx, "UpdateByID")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	message, err := s.queries.UpdateById(traceCtx, UpdateByIdParams{
		ID:         id,
		UserID:     userId,
		IsRead:     arg.IsRead,
		IsArchived: arg.IsArchived,
		IsStarred:  arg.IsStarred,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "update user_inbox_message by id")
		span.RecordError(err)
		return UpdateByIdRow{}, err
	}

	return message, err
}
