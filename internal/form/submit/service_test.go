package submit

import (
	"context"
	"errors"
	"testing"

	"NYCU-SDC/core-system-backend/internal/form/question"
	"NYCU-SDC/core-system-backend/internal/form/response"
	"NYCU-SDC/core-system-backend/internal/form/shared"

	"NYCU-SDC/core-system-backend/internal/form/submit/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeAnswerable struct {
	q           question.Question
	validateErr error
}

func (f fakeAnswerable) Question() question.Question {
	return f.q
}
func (f fakeAnswerable) Validate(value string) error {
	return f.validateErr
}

func ans(qID uuid.UUID, val string) shared.AnswerParam {
	return shared.AnswerParam{QuestionID: qID.String(), Value: val}
}

func makeQ(questionID uuid.UUID, t question.QuestionType, validateErr error) question.Answerable {
	return fakeAnswerable{
		q: question.Question{
			ID:   questionID,
			Type: t,
		},
		validateErr: validateErr,
	}
}

func TestSubmitService_Submit(t *testing.T) {
	type Params struct {
		formID  uuid.UUID
		userID  uuid.UUID
		answers []shared.AnswerParam
		qs      []question.Answerable

		qStore *mocks.QuestionStore
		rStore *mocks.FormResponseStore
		svc    *Service
	}

	type testCase struct {
		name        string
		params      Params
		expected    response.FormResponse
		expectedErr bool // meaning: errs slice should be non-empty
		setup       func(t *testing.T, p *Params) context.Context
		validate    func(t *testing.T, p Params, got response.FormResponse, errs []error)
	}

	formID := uuid.New()
	userID := uuid.New()
	q1, q2 := uuid.New(), uuid.New()
	unknown := uuid.New()

	testCases := []testCase{
		{
			name: "All valid → CreateOrUpdate is called with mapped types",
			params: Params{
				formID:  formID,
				userID:  userID,
				qs:      []question.Answerable{makeQ(q1, question.QuestionTypeShortText, nil), makeQ(q2, question.QuestionTypeLongText, nil)},
				answers: []shared.AnswerParam{ans(q1, "hello"), ans(q2, "world")},
			},
			setup: func(t *testing.T, p *Params) context.Context {
				qm := mocks.NewQuestionStore(t)
				rm := mocks.NewFormResponseStore(t)

				qm.EXPECT().
					ListByFormID(mock.Anything, p.formID).
					Return(p.qs, nil).Once()

				respID := uuid.New()
				rm.EXPECT().
					CreateOrUpdate(
						mock.Anything,
						p.formID,
						p.userID,
						p.answers,
						[]response.QuestionType{
							response.QuestionType(question.QuestionTypeShortText),
							response.QuestionType(question.QuestionTypeLongText),
						},
					).
					Return(response.FormResponse{ID: respID, FormID: p.formID, SubmittedBy: p.userID}, nil).
					Once()

				p.qStore = qm
				p.rStore = rm
				p.svc = NewService(zap.NewNop(), qm, rm)
				return context.Background()
			},
			validate: func(t *testing.T, p Params, got response.FormResponse, errs []error) {
				require.Nil(t, errs)
				require.Equal(t, p.formID, got.FormID)
				require.Equal(t, p.userID, got.SubmittedBy)
				require.NotEqual(t, uuid.Nil, got.ID)
			},
		},
		{
			name: "Some invalid answers → do not call CreateOrUpdate",
			params: Params{
				formID:  formID,
				userID:  userID,
				qs:      []question.Answerable{makeQ(q1, question.QuestionTypeShortText, nil), makeQ(q2, question.QuestionTypeLongText, errors.New("invalid"))},
				answers: []shared.AnswerParam{ans(q1, "ok"), ans(q2, "bad")},
			},
			expectedErr: true,
			setup: func(t *testing.T, p *Params) context.Context {
				qm := mocks.NewQuestionStore(t)
				rm := mocks.NewFormResponseStore(t)

				qm.EXPECT().
					ListByFormID(mock.Anything, p.formID).
					Return(p.qs, nil).Once()

				p.qStore = qm
				p.rStore = rm
				p.svc = NewService(zap.NewNop(), qm, rm)
				return context.Background()
			},
			validate: func(t *testing.T, p Params, got response.FormResponse, errs []error) {
				require.NotNil(t, errs)
				require.Len(t, errs, 1)
				require.Equal(t, uuid.Nil, got.ID)
				p.rStore.AssertNotCalled(t, "CreateOrUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name: "Answer refers to unknown question → do not call CreateOrUpdate",
			params: Params{
				formID:  formID,
				userID:  userID,
				qs:      []question.Answerable{makeQ(q1, question.QuestionTypeShortText, nil)},
				answers: []shared.AnswerParam{ans(q1, "ok"), ans(unknown, "???")},
			},
			expectedErr: true,
			setup: func(t *testing.T, p *Params) context.Context {
				qm := mocks.NewQuestionStore(t)
				rm := mocks.NewFormResponseStore(t)

				qm.EXPECT().
					ListByFormID(mock.Anything, p.formID).
					Return(p.qs, nil).Once()

				p.qStore = qm
				p.rStore = rm
				p.svc = NewService(zap.NewNop(), qm, rm)
				return context.Background()
			},
			validate: func(t *testing.T, p Params, got response.FormResponse, errs []error) {
				require.NotNil(t, errs)
				require.Len(t, errs, 1)
				require.Equal(t, uuid.Nil, got.ID)
				p.rStore.AssertNotCalled(t, "CreateOrUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name: "ListByFormID fails → return error and do not call CreateOrUpdate",
			params: Params{
				formID:  formID,
				userID:  userID,
				answers: []shared.AnswerParam{ans(q1, "x")},
			},
			expectedErr: true,
			setup: func(t *testing.T, p *Params) context.Context {
				qm := mocks.NewQuestionStore(t)
				rm := mocks.NewFormResponseStore(t)

				qm.EXPECT().
					ListByFormID(mock.Anything, p.formID).
					Return(nil, errors.New("db down")).Once()

				p.qStore = qm
				p.rStore = rm
				p.svc = NewService(zap.NewNop(), qm, rm)
				return context.Background()
			},
			validate: func(t *testing.T, p Params, got response.FormResponse, errs []error) {
				require.NotNil(t, errs)
				require.Equal(t, uuid.Nil, got.ID)
				p.rStore.AssertNotCalled(t, "CreateOrUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name: "Validation passes but CreateOrUpdate fails → return error",
			params: Params{
				formID:  formID,
				userID:  userID,
				qs:      []question.Answerable{makeQ(q1, question.QuestionTypeShortText, nil)},
				answers: []shared.AnswerParam{ans(q1, "ok")},
			},
			expectedErr: true,
			setup: func(t *testing.T, p *Params) context.Context {
				qm := mocks.NewQuestionStore(t)
				rm := mocks.NewFormResponseStore(t)

				qm.EXPECT().
					ListByFormID(mock.Anything, p.formID).
					Return(p.qs, nil).Once()

				rm.EXPECT().
					CreateOrUpdate(
						mock.Anything,
						p.formID,
						p.userID,
						p.answers,
						[]response.QuestionType{response.QuestionType(question.QuestionTypeShortText)},
					).
					Return(response.FormResponse{}, errors.New("insert fail")).
					Once()

				p.qStore = qm
				p.rStore = rm
				p.svc = NewService(zap.NewNop(), qm, rm)
				return context.Background()
			},
			validate: func(t *testing.T, p Params, got response.FormResponse, errs []error) {
				require.NotNil(t, errs)
				require.Equal(t, uuid.Nil, got.ID)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			params := tc.params
			if tc.setup != nil {
				ctx = tc.setup(t, &params)
			}

			got, errs := params.svc.Submit(ctx, params.formID, params.userID, params.answers)
			require.Equal(t, tc.expectedErr, len(errs) > 0, "expectedErr=%v, errs=%v", tc.expectedErr, errs)

			if tc.validate != nil {
				tc.validate(t, params, got, errs)
			}
		})
	}
}
