package workflow

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
	Get(ctx context.Context, formID uuid.UUID) (GetRow, error)
	Update(ctx context.Context, arg UpdateParams) (UpdateRow, error)
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
		tracer:  otel.Tracer("workflow/service"),
	}
}

// Get retrieves the latest workflow version for a form
func (s *Service) Get(ctx context.Context, formID uuid.UUID) (GetRow, error) {
	methodName := "Get"
	ctx, span := s.tracer.Start(ctx, methodName)
	defer span.End()
	logger := logutil.WithContext(ctx, s.logger)

	workflow, err := s.queries.Get(ctx, formID)
	if err != nil {
		err = databaseutil.WrapDBErrorWithKeyValue(err, "workflow", "formId", formID.String(), logger, "get workflow by form id")
		span.RecordError(err)
		return GetRow{}, err
	}

	return workflow, nil
}

// Update updates a workflow version conditionally:
// - If latest workflow is active: creates a new workflow version
// - If latest workflow is draft: updates the existing workflow version
func (s *Service) Update(ctx context.Context, formID uuid.UUID, workflow []byte, userID uuid.UUID) (UpdateRow, error) {
	methodName := "Update"
	ctx, span := s.tracer.Start(ctx, methodName)
	defer span.End()
	logger := logutil.WithContext(ctx, s.logger)

	// Basic JSON validation - check if workflow is valid JSON
	// TODO: More detailed graph validation would be added later
	if len(workflow) == 0 {
		workflow = []byte("[]")
	}

	updated, err := s.queries.Update(ctx, UpdateParams{
		FormID:     formID,
		LastEditor: userID,
		Workflow:   workflow,
	})
	if err != nil {
		err = databaseutil.WrapDBErrorWithKeyValue(err, "workflow", "formId", formID.String(), logger, "update workflow")
		span.RecordError(err)
		return UpdateRow{}, err
	}

	return updated, nil
}
