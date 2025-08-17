package inbox

import (
	"NYCU-SDC/core-system-backend/internal"
	"NYCU-SDC/core-system-backend/internal/form"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"

	databaseutil "github.com/NYCU-SDC/summer/pkg/database"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type ContentQuerier struct {
	Form form.Querier
}

type Querier interface {
	GetAll(ctx context.Context, userID pgtype.UUID) ([]GetAllRow, error)
	GetById(ctx context.Context, arg GetByIdParams) (GetByIdRow, error)
	UpdateById(ctx context.Context, arg UpdateByIdParams) (UpdateByIdRow, error)
}
type Service struct {
	logger  *zap.Logger
	queries Querier
	tracer  trace.Tracer

	contentQuerier ContentQuerier
}

func NewService(logger *zap.Logger, db DBTX) *Service {
	return &Service{
		logger:  logger,
		queries: New(db),
		tracer:  otel.Tracer("inbox/service"),
		contentQuerier: ContentQuerier{
			Form: form.New(db),
		},
	}
}

func (s *Service) GetMessageContent(ctx context.Context, contentType ContentType, contentId uuid.UUID) (any, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetMessageContent")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	switch contentType {
	case ContentTypeForm:
		currentForm, err := s.contentQuerier.Form.GetByID(traceCtx, contentId)
		if err != nil {
			err = databaseutil.WrapDBError(err, logger, "get form by id")
			span.RecordError(err)
			return Form{}, err
		}
		return currentForm, nil
	case ContentTypeText:
		return nil, nil
	}

	return nil, fmt.Errorf("content type %s not supported", contentType)
}

func (s *Service) GetAll(ctx context.Context, userId uuid.UUID) ([]GetAllRow, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetAll")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	messages, err := s.queries.GetAll(traceCtx, pgtype.UUID{Bytes: userId, Valid: true})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "get all user inbox messages")
		span.RecordError(err)
		return nil, err
	}

	if messages == nil {
		return []GetAllRow{}, err
	}

	return messages, err
}

func (s *Service) GetById(ctx context.Context, id uuid.UUID, userId uuid.UUID) (GetByIdRow, any, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetById")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	message, err := s.queries.GetById(traceCtx, GetByIdParams{
		ID:     id,
		UserID: pgtype.UUID{Bytes: userId, Valid: true},
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "get the full message by id")
		span.RecordError(err)
		return GetByIdRow{}, nil, err
	}

	contentId, err := internal.ParseUUID(message.ContentID.String())
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "parse the messageId string into uuid")
		span.RecordError(err)
		return message, nil, err
	}

	content, err := s.GetMessageContent(traceCtx, message.Type, contentId)
	if err != nil {
		return message, nil, err
	}

	return message, content, err
}

func (s *Service) UpdateById(ctx context.Context, id uuid.UUID, userId uuid.UUID, arg UserInboxMessageFilter) (UpdateByIdRow, error) {
	traceCtx, span := s.tracer.Start(ctx, "UpdateById")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	message, err := s.queries.UpdateById(traceCtx, UpdateByIdParams{
		ID:         id,
		UserID:     pgtype.UUID{Bytes: userId, Valid: true},
		IsRead:     pgtype.Bool{Bool: arg.IsRead, Valid: true},
		IsArchived: pgtype.Bool{Bool: arg.IsArchived, Valid: true},
		IsStarred:  pgtype.Bool{Bool: arg.IsStarred, Valid: true},
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "update message by id")
		span.RecordError(err)
		return UpdateByIdRow{}, err
	}

	return message, err
}
