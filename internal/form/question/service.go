package question

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
	Create(ctx context.Context, params CreateParams) (Question, error)
	Update(ctx context.Context, params UpdateParams) (Question, error)
	Delete(ctx context.Context, params DeleteParams) error
	ListByFormID(ctx context.Context, formID uuid.UUID) ([]ListByFormIDRow, error)
	GetByID(ctx context.Context, id uuid.UUID) (Question, error)
}

type Answerable interface {
	Question() Question
	Validate(value string) error
}

type SectionWithQuestions struct {
	Section   Section
	Questions []Answerable
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
		tracer:  otel.Tracer("question/service"),
	}
}

func (s *Service) Create(ctx context.Context, input CreateParams) (Answerable, error) {
	ctx, span := s.tracer.Start(ctx, "Create")
	defer span.End()
	logger := logutil.WithContext(ctx, s.logger)

	q, err := s.queries.Create(ctx, input)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "create question")
		span.RecordError(err)
		return nil, err
	}

	return NewAnswerable(q)
}

func (s *Service) Update(ctx context.Context, input UpdateParams) (Answerable, error) {
	ctx, span := s.tracer.Start(ctx, "Update")
	defer span.End()
	logger := logutil.WithContext(ctx, s.logger)

	q, err := s.queries.Update(ctx, input)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "update question")
		span.RecordError(err)
		return nil, err
	}

	return NewAnswerable(q)
}

func (s *Service) Delete(ctx context.Context, sectionID uuid.UUID, id uuid.UUID) error {
	ctx, span := s.tracer.Start(ctx, "Delete")
	defer span.End()
	logger := logutil.WithContext(ctx, s.logger)

	err := s.queries.Delete(ctx, DeleteParams{
		SectionID: sectionID,
		ID:        id,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "delete question")
		span.RecordError(err)
		return err
	}

	return nil
}

func (s *Service) ListByFormID(ctx context.Context, formID uuid.UUID) ([]SectionWithQuestions, error) {
	ctx, span := s.tracer.Start(ctx, "ListByFormID")
	defer span.End()
	logger := logutil.WithContext(ctx, s.logger)

	list, err := s.queries.ListByFormID(ctx, formID)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "list questions by form id")
		span.RecordError(err)
		return nil, err
	}

	sectionMap := make(map[uuid.UUID]*SectionWithQuestions)
	for _, row := range list {
		sectionID := row.SectionID

		_, exist := sectionMap[sectionID]
		if !exist {
			sectionMap[sectionID] = &SectionWithQuestions{
				Section: Section{
					ID:          sectionID,
					FormID:      row.FormID,
					Title:       row.Title,
					Description: row.Description,
					CreatedAt:   row.CreatedAt,
					UpdatedAt:   row.UpdatedAt,
				},
				Questions: []Answerable{},
			}
		}

		// Check if question exists
		if row.ID.Valid {
			q := Question{
				ID:          row.ID.Bytes,
				SectionID:   sectionID,
				Required:    row.Required.Bool,
				Type:        row.Type.QuestionType,
				Title:       row.QuestionTitle,
				Description: row.QuestionDescription,
				Metadata:    row.Metadata,
				Order:       row.Order.Int32,
				SourceID:    row.SourceID,
				CreatedAt:   row.QuestionCreatedAt,
				UpdatedAt:   row.QuestionUpdatedAt,
			}
			answerable, err := NewAnswerable(q)
			if err != nil {
				err = databaseutil.WrapDBError(err, logger, "create answerable from question")
				span.RecordError(err)
				return nil, err
			}

			sectionMap[sectionID].Questions = append(sectionMap[sectionID].Questions, answerable)
		}
	}

	result := make([]SectionWithQuestions, 0, len(sectionMap))
	for _, q := range sectionMap {
		result = append(result, *q)
	}
	return result, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (Answerable, error) {
	ctx, span := s.tracer.Start(ctx, "GetByID")
	defer span.End()
	logger := logutil.WithContext(ctx, s.logger)

	q, err := s.queries.GetByID(ctx, id)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "get question by id")
		span.RecordError(err)
		return nil, err
	}

	return NewAnswerable(q)
}
