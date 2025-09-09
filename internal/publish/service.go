package publish

import (
	"context"
	"errors"

	databaseutil "github.com/NYCU-SDC/summer/pkg/database"
	logutil "github.com/NYCU-SDC/summer/pkg/log"

	"NYCU-SDC/core-system-backend/internal/form"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Distributor interface {
	GetRecipients(ctx context.Context, orgID, unitIDs []uuid.UUID) ([]uuid.UUID, error)
}

type FormStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (form.Form, error)
	SetStatus(ctx context.Context, id uuid.UUID, status string, editor uuid.UUID) error
}

type InboxPort interface {
	CreateOrUpdateThread(ctx context.Context, f form.Form, users []uuid.UUID)
}

type Selection struct {
	OrgID   []uuid.UUID
	UnitIDs []uuid.UUID
}

type Service struct {
	logger      *zap.Logger
	tracer      trace.Tracer
	distributor Distributor
	store       FormStore
	inbox       InboxPort
}

func NewService(
	logger *zap.Logger,
	distributor Distributor,
	store FormStore,
	inbox InboxPort,
) *Service {
	return &Service{
		logger:      logger,
		tracer:      otel.Tracer("publish/service"),
		distributor: distributor,
		store:       store,
		inbox:       inbox,
	}
}

func (s *Service) PreviewRecipients(ctx context.Context, sel Selection) ([]uuid.UUID, error) {
	ctx, span := s.tracer.Start(ctx, "PreviewRecipients")
	defer span.End()
	logger := logutil.WithContext(ctx, s.logger)

	users, err := s.distributor.GetRecipients(ctx, sel.OrgID, sel.UnitIDs)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "getting recipients")
		span.RecordError(err)
		return nil, err
	}

	// can add some verify method here

	return users, nil
}

// PublishForm not Publish is because maybe we will publish something else in future
func (s *Service) PublishForm(ctx context.Context, formID uuid.UUID, sel Selection, editor uuid.UUID) error {
	ctx, span := s.tracer.Start(ctx, "PublishForm")
	defer span.End()
	logger := logutil.WithContext(ctx, s.logger)

	// get form
	f, err := s.store.GetByID(ctx, formID)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "getting form by id")
		span.RecordError(err)
		return err
	}

	if string(f.Status) == "published" {
		// do something here?
	}

	// get recipients
	ids, err := s.distributor.GetRecipients(ctx, sel.OrgID, sel.UnitIDs)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "get recipients")
		span.RecordError(err)
		return err
	}
	if len(ids) == 0 {
		// 再改一下error?
		return errors.New("no recipients selected")
	}

	// set status
	if err := s.store.SetStatus(ctx, f.ID, "published", editor); err != nil {
		err = databaseutil.WrapDBError(err, logger, "setting form status = published")
		span.RecordError(err)
		return err
	}

	// update inbox thread
	if err := s.inbox.CreateOrUpdateThread(ctx, f, ids); err != nil {
		err = databaseutil.WrapDBError(err, logger, "creating/update inbox thread")
		span.RecordError(err)
		return err
	}

	logger.Info("Form published",
		zap.String("form_id", f.ID.String()),
		zap.String("recipients", len(ids)),
		zap.String("editor", editor.String()),
	)
	return nil
}
