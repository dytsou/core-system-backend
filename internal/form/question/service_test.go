package question_test

import (
	"context"
	"testing"

	"NYCU-SDC/core-system-backend/internal/form/question"
	"NYCU-SDC/core-system-backend/internal/form/question/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func mkQuestion(t question.QuestionType) question.Question {
	q := question.Question{Type: t}

	switch t {
	case question.QuestionTypeSingleChoice:
		md, _ := question.GenerateMetadata("single_choice", []question.ChoiceOption{{Name: "A"}, {Name: "B"}})
		q.Metadata = md
	case question.QuestionTypeMultipleChoice:
		md, _ := question.GenerateMetadata("multiple_choice", []question.ChoiceOption{{Name: "A"}, {Name: "B"}})
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
		createReturn question.Question
		wantErr      bool
	}{
		{
			name:         "Known type (ShortText) -> success",
			createReturn: mkQuestion(question.QuestionTypeShortText),
			wantErr:      false,
		},
		{
			name:         "Known type (LongText) -> success)",
			createReturn: mkQuestion(question.QuestionTypeLongText),
			wantErr:      false,
		},
		{
			name:         "Known type (SingleChoice) -> success)",
			createReturn: mkQuestion(question.QuestionTypeSingleChoice),
			wantErr:      false,
		},
		{
			name:         "Known type (MultipleChoice) -> success)",
			createReturn: mkQuestion(question.QuestionTypeMultipleChoice),
			wantErr:      false,
		},
		{
			name:         "Known type (Date) -> success)",
			createReturn: mkQuestion(question.QuestionTypeDate),
			wantErr:      false,
		},
		{
			name:         "Unknown type (Unknown) -> error",
			createReturn: mkQuestion(question.QuestionType("___UNKNOWN___")),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			mq := mocks.NewMockQuerier(t)
			mq.EXPECT().
				Create(mock.Anything, question.CreateParams{}).
				Return(tt.createReturn, nil).Once()
			logger := zap.NewNop()
			svc := question.NewService(logger, mq)

			got, err := svc.Create(ctx, question.CreateParams{})
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

	allKnown := []question.Question{
		mkQuestion(question.QuestionTypeShortText),
		mkQuestion(question.QuestionTypeSingleChoice),
	}

	withUnknown := []question.Question{
		mkQuestion(question.QuestionTypeLongText),
		mkQuestion(question.QuestionType("___UNKNOWN___")),
	}

	tests := []struct {
		name       string
		listReturn []question.Question
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
			mq := mocks.NewMockQuerier(t)
			mq.EXPECT().
				ListByFormID(mock.Anything, formID).
				Return(tt.listReturn, nil).Once()

			logger := zap.NewNop()
			svc := question.NewService(logger, mq)

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
