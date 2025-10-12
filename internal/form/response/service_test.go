package response

import (
	"context"
	"errors"
	"testing"

	"NYCU-SDC/core-system-backend/internal/form/response/mocks"
	"NYCU-SDC/core-system-backend/internal/form/shared"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

func newSvcWithMock(q Querier) *Service {
	return &Service{
		logger:  zap.NewNop(),
		queries: q,
		tracer:  otel.Tracer("test"),
	}
}

func ap(qID uuid.UUID, val string) shared.AnswerParam {
	return shared.AnswerParam{
		QuestionID: qID.String(),
		Value:      val,
	}
}

func TestService_CreateOrUpdate(t *testing.T) {
	type Params struct {
		formID uuid.UUID
		userID uuid.UUID
		ans    []shared.AnswerParam
		types  []QuestionType

		svc  *Service
		mock *mocks.Querier
	}

	type testCase struct {
		name        string
		params      Params
		expected    FormResponse
		expectedErr bool
		setup       func(t *testing.T, p *Params) context.Context
		validate    func(t *testing.T, p Params, got FormResponse)
	}

	testCases := []testCase{
		{
			name: "Return error when answers length != question types length",
			params: Params{
				formID: uuid.New(),
				userID: uuid.New(),
				ans:    []shared.AnswerParam{ap(uuid.New(), "A")}, // len=1
				types:  []QuestionType{},                          // len=0
			},
			expectedErr: true,
			setup: func(t *testing.T, p *Params) context.Context {
				q := mocks.NewQuerier(t)
				p.mock = q
				p.svc = newSvcWithMock(q)
				return context.Background()
			},
		},
		{
			name: "Exists=false → Create response and answers",
			params: Params{
				formID: uuid.New(),
				userID: uuid.New(),
			},
			setup: func(t *testing.T, p *Params) context.Context {
				respID := uuid.New()
				q1, q2 := uuid.New(), uuid.New()

				p.ans = []shared.AnswerParam{ap(q1, "v1"), ap(q2, "v2")}
				p.types = []QuestionType{QuestionTypeShortText, QuestionTypeLongText}

				q := mocks.NewQuerier(t)

				q.EXPECT().
					Exists(mock.Anything, ExistsParams{FormID: p.formID, SubmittedBy: p.userID}).
					Return(false, nil).Once()

				q.EXPECT().
					Create(mock.Anything, mock.MatchedBy(func(cp CreateParams) bool {
						return cp.FormID == p.formID && cp.SubmittedBy == p.userID
					})).
					Return(FormResponse{ID: respID, FormID: p.formID, SubmittedBy: p.userID}, nil).Once()

				q.EXPECT().
					CreateAnswer(mock.Anything, mock.MatchedBy(func(a CreateAnswerParams) bool {
						return a.ResponseID == respID && a.QuestionID == q1 && a.Value == "v1"
					})).
					Return(Answer{ID: uuid.New(), ResponseID: respID, QuestionID: q1}, nil).Once()

				q.EXPECT().
					CreateAnswer(mock.Anything, mock.MatchedBy(func(a CreateAnswerParams) bool {
						return a.ResponseID == respID && a.QuestionID == q2 && a.Value == "v2"
					})).
					Return(Answer{ID: uuid.New(), ResponseID: respID, QuestionID: q2}, nil).Once()

				p.mock = q
				p.svc = newSvcWithMock(q)
				return context.Background()
			},
			validate: func(t *testing.T, p Params, got FormResponse) {
				require.Equal(t, p.formID, got.FormID)
				require.Equal(t, p.userID, got.SubmittedBy)
				require.NotEqual(t, uuid.Nil, got.ID)
			},
		},
		{
			name: "Update if it exists",
			params: Params{
				formID: uuid.New(),
				userID: uuid.New(),
				ans:    []shared.AnswerParam{},
				types:  []QuestionType{},
			},
			setup: func(t *testing.T, p *Params) context.Context {
				respID := uuid.New()
				q := mocks.NewQuerier(t)

				q.EXPECT().
					Exists(mock.Anything, ExistsParams{FormID: p.formID, SubmittedBy: p.userID}).
					Return(true, nil).Once()

				q.EXPECT().
					GetByFormIDAndSubmittedBy(mock.Anything, GetByFormIDAndSubmittedByParams{
						FormID: p.formID, SubmittedBy: p.userID,
					}).
					Return(FormResponse{ID: respID, FormID: p.formID, SubmittedBy: p.userID}, nil).Once()

				q.EXPECT().Update(mock.Anything, respID).Return(nil).Once()

				p.mock = q
				p.svc = newSvcWithMock(q)
				return context.Background()
			},
			validate: func(t *testing.T, p Params, got FormResponse) {
				require.Equal(t, p.formID, got.FormID)
				require.Equal(t, p.userID, got.SubmittedBy)
			},
		},
		{
			name: "Exists query fails → return error",
			params: Params{
				formID: uuid.New(),
				userID: uuid.New(),
			},
			expectedErr: true,
			setup: func(t *testing.T, p *Params) context.Context {
				q := mocks.NewQuerier(t)
				q.EXPECT().
					Exists(mock.Anything, ExistsParams{FormID: p.formID, SubmittedBy: p.userID}).
					Return(false, errors.New("db down")).Once()
				p.mock = q
				p.svc = newSvcWithMock(q)
				return context.Background()
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

			result, err := params.svc.CreateOrUpdate(ctx, params.formID, params.userID, params.ans, params.types)
			require.Equal(t, tc.expectedErr, err != nil, "expected error: %v, got: %v", tc.expectedErr, err)

			if tc.expectedErr {
				return
			}
			if tc.validate != nil {
				tc.validate(t, params, result)
			}
		})
	}
}

func TestService_Update(t *testing.T) {
	type Params struct {
		formID uuid.UUID
		userID uuid.UUID
		ans    []shared.AnswerParam
		types  []QuestionType

		respID uuid.UUID
		qNew   uuid.UUID
		qSame  uuid.UUID
		qDiff  uuid.UUID

		svc  *Service
		mock *mocks.Querier
	}

	type testCase struct {
		name        string
		params      Params
		expected    FormResponse
		expectedErr bool
		setup       func(t *testing.T, p *Params) context.Context
		validate    func(t *testing.T, p Params, got FormResponse)
	}

	testCases := []testCase{
		{
			name: "Create answer when it does not exist",
			params: Params{
				formID: uuid.New(),
				userID: uuid.New(),
				qNew:   uuid.New(),
			},
			setup: func(t *testing.T, p *Params) context.Context {
				p.respID = uuid.New()
				p.ans = []shared.AnswerParam{ap(p.qNew, "aaa")}
				p.types = []QuestionType{QuestionTypeShortText}

				q := mocks.NewQuerier(t)

				q.EXPECT().
					GetByFormIDAndSubmittedBy(mock.Anything, GetByFormIDAndSubmittedByParams{
						FormID: p.formID, SubmittedBy: p.userID,
					}).
					Return(FormResponse{ID: p.respID, FormID: p.formID, SubmittedBy: p.userID}, nil).Once()

				q.EXPECT().
					AnswerExists(mock.Anything, AnswerExistsParams{ResponseID: p.respID, QuestionID: p.qNew}).
					Return(false, nil).Once()

				q.EXPECT().
					CreateAnswer(mock.Anything, mock.MatchedBy(func(a CreateAnswerParams) bool {
						return a.ResponseID == p.respID && a.QuestionID == p.qNew && a.Value == "aaa"
					})).
					Return(Answer{ID: uuid.New(), ResponseID: p.respID, QuestionID: p.qNew}, nil).Once()

				q.EXPECT().Update(mock.Anything, p.respID).Return(nil).Once()

				p.mock = q
				p.svc = newSvcWithMock(q)
				return context.Background()
			},
			validate: func(t *testing.T, p Params, got FormResponse) {
				require.Equal(t, p.formID, got.FormID)
				require.Equal(t, p.userID, got.SubmittedBy)
			},
		},
		{
			name: "Do nothing when answer exists and content is the same",
			params: Params{
				formID: uuid.New(),
				userID: uuid.New(),
				qSame:  uuid.New(),
			},
			setup: func(t *testing.T, p *Params) context.Context {
				p.respID = uuid.New()
				p.ans = []shared.AnswerParam{ap(p.qSame, "bbb")}
				p.types = []QuestionType{QuestionTypeShortText}

				q := mocks.NewQuerier(t)

				q.EXPECT().
					GetByFormIDAndSubmittedBy(mock.Anything, GetByFormIDAndSubmittedByParams{
						FormID: p.formID, SubmittedBy: p.userID,
					}).
					Return(FormResponse{ID: p.respID, FormID: p.formID, SubmittedBy: p.userID}, nil).Once()

				q.EXPECT().
					AnswerExists(mock.Anything, AnswerExistsParams{ResponseID: p.respID, QuestionID: p.qSame}).
					Return(true, nil).Once()

				q.EXPECT().
					CheckAnswerContent(mock.Anything, CheckAnswerContentParams{
						ResponseID: p.respID, QuestionID: p.qSame, Value: "bbb",
					}).
					Return(true, nil).Once()

				q.EXPECT().Update(mock.Anything, p.respID).Return(nil).Once()

				p.mock = q
				p.svc = newSvcWithMock(q)
				return context.Background()
			},
			validate: func(t *testing.T, p Params, got FormResponse) {
				require.Equal(t, p.formID, got.FormID)
				require.Equal(t, p.userID, got.SubmittedBy)
				p.mock.AssertNotCalled(t, "UpdateAnswer", mock.Anything, mock.Anything)
			},
		},
		{
			name: "Update answer when it exists and content is different",
			params: Params{
				formID: uuid.New(),
				userID: uuid.New(),
				qDiff:  uuid.New(),
			},
			setup: func(t *testing.T, p *Params) context.Context {
				p.respID = uuid.New()
				p.ans = []shared.AnswerParam{ap(p.qDiff, "ccc")}
				p.types = []QuestionType{QuestionTypeShortText}
				ansID := uuid.New()

				q := mocks.NewQuerier(t)

				q.EXPECT().
					GetByFormIDAndSubmittedBy(mock.Anything, GetByFormIDAndSubmittedByParams{
						FormID: p.formID, SubmittedBy: p.userID,
					}).
					Return(FormResponse{ID: p.respID, FormID: p.formID, SubmittedBy: p.userID}, nil).Once()

				q.EXPECT().
					AnswerExists(mock.Anything, AnswerExistsParams{ResponseID: p.respID, QuestionID: p.qDiff}).
					Return(true, nil).Once()

				q.EXPECT().
					CheckAnswerContent(mock.Anything, CheckAnswerContentParams{
						ResponseID: p.respID, QuestionID: p.qDiff, Value: "ccc",
					}).
					Return(false, nil).Once()

				q.EXPECT().
					GetAnswerID(mock.Anything, GetAnswerIDParams{ResponseID: p.respID, QuestionID: p.qDiff}).
					Return(ansID, nil).Once()

				q.EXPECT().
					UpdateAnswer(mock.Anything, UpdateAnswerParams{ID: ansID, Value: "ccc"}).
					Return(Answer{ID: ansID, Value: "ccc"}, nil).Once()

				q.EXPECT().Update(mock.Anything, p.respID).Return(nil).Once()

				p.mock = q
				p.svc = newSvcWithMock(q)
				return context.Background()
			},
			validate: func(t *testing.T, p Params, got FormResponse) {
				require.Equal(t, p.formID, got.FormID)
				require.Equal(t, p.userID, got.SubmittedBy)
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

			result, err := params.svc.Update(ctx, params.formID, params.userID, params.ans, params.types)
			require.Equal(t, tc.expectedErr, err != nil, "expected error: %v, got: %v", tc.expectedErr, err)

			if tc.expectedErr {
				return
			}
			if tc.validate != nil {
				tc.validate(t, params, result)
			}
		})
	}
}
