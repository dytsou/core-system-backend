package question

import (
	"context"
	"testing"

	"NYCU-SDC/core-system-backend/internal/form/question/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

func mkQuestion(t QuestionType) Question {
	q := Question{Type: t}

	switch t {
	case QuestionTypeSingleChoice:
		md, _ := GenerateMetadata("single_choice", []ChoiceOption{{Name: "A"}, {Name: "B"}})
		q.Metadata = md
	case QuestionTypeMultipleChoice:
		md, _ := GenerateMetadata("multiple_choice", []ChoiceOption{{Name: "A"}, {Name: "B"}})
		q.Metadata = md
	default:
		q.Metadata = []byte(`{}`)
	}
	return q
}

func TestService_Create_KnownAndUnknown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		createReturn Question
		wantErr      bool
	}{
		{
			name:         "Known type (ShortText) -> success",
			createReturn: mkQuestion(QuestionTypeShortText),
			wantErr:      false,
		},
		{
			name:         "Known type (LongText) -> success)",
			createReturn: mkQuestion(QuestionTypeLongText),
			wantErr:      false,
		},
		{
			name:         "Known type (SingleChoice) -> success)",
			createReturn: mkQuestion(QuestionTypeSingleChoice),
			wantErr:      false,
		},
		{
			name:         "Known type (MultipleChoice) -> success)",
			createReturn: mkQuestion(QuestionTypeMultipleChoice),
			wantErr:      false,
		},
		{
			name:         "Known type (Date) -> success)",
			createReturn: mkQuestion(QuestionTypeDate),
			wantErr:      false,
		},
		{
			name:         "Unknown type (Unknown) -> error",
			createReturn: mkQuestion(QuestionType("___UNKNOWN___")),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			mq := mocks.NewQuerier(t)
			mq.EXPECT().
				Create(mock.Anything, CreateParams{}).
				Return(tt.createReturn, nil).Once()
			logger := zap.NewNop()
			svc := &Service{
				logger:  logger,
				queries: mq,
				tracer:  otel.Tracer("test"),
			}

			got, err := svc.Create(ctx, CreateParams{})
			if tt.wantErr {
				require.Error(t, err, "expected error but got nil")
				require.Nil(t, got)
				return
			}

			require.NoError(t, err, "unexpected error occurred")
			require.NotNil(t, got, "should return an Answerable")
			require.Equal(t, tt.createReturn.Type, got.Question().Type)
		})
	}
}

func TestService_ListByFormID_AllKnown_And_ContainsUnknown(t *testing.T) {
	t.Parallel()

	formID := uuid.New()

	allKnown := []Question{
		mkQuestion(QuestionTypeShortText),
		mkQuestion(QuestionTypeSingleChoice),
	}

	withUnknown := []Question{
		mkQuestion(QuestionTypeLongText),
		mkQuestion(QuestionType("___UNKNOWN___")),
	}

	tests := []struct {
		name       string
		listReturn []Question
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "All known types -> return []Answerable",
			listReturn: allKnown,
			wantCount:  len(allKnown),
			wantErr:    false,
		},
		{
			name:       "Contains an unknown type -> fail",
			listReturn: withUnknown,
			wantCount:  0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			mq := mocks.NewQuerier(t)
			mq.EXPECT().
				ListByFormID(mock.Anything, formID).
				Return(tt.listReturn, nil).Once()

			logger := zap.NewNop()
			svc := &Service{
				logger:  logger,
				queries: mq,
				tracer:  otel.Tracer("test"),
			}

			got, err := svc.ListByFormID(ctx, formID)

			if tt.wantErr {
				require.Error(t, err, "expected error but got nil")
				require.Nil(t, got)
				return
			}

			require.NoError(t, err, "unexpected error")
			require.Len(t, got, tt.wantCount)

			for i, a := range got {
				require.NotNil(t, a, "answerable[%d] should not be nil", i)
				require.Equal(t, tt.listReturn[i].Type, a.Question().Type)
			}
		})
	}
}
