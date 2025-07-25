package response

import (
	"context"
	"net/http"
	"time"

	"NYCU-SDC/core-system-backend/internal"
	"NYCU-SDC/core-system-backend/internal/form/question"
	"NYCU-SDC/core-system-backend/internal/user"

	handlerutil "github.com/NYCU-SDC/summer/pkg/handler"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/NYCU-SDC/summer/pkg/problem"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type AnswerRequest struct {
	QuestionID string `json:"questionId" validate:"required,uuid"`
	Value      string `json:"value" validate:"required"`
}

type AnswerResponse struct {
	ID         string                `json:"id" validate:"required,uuid"`
	ResponseID string                `json:"responseId" validate:"required,uuid"`
	QuestionID string                `json:"questionId" validate:"required,uuid"`
	Type       question.QuestionType `json:"type" validate:"required,oneof=shortText longText singleChoice multipleChoice date"`
	Value      string                `json:"value" validate:"required"`
	CreatedAt  time.Time             `json:"createdAt" validate:"required,datetime"`
	UpdatedAt  time.Time             `json:"updatedAt" validate:"required,datetime"`
}

type SubmitRequest struct {
	Answers []AnswerRequest `json:"answers" validate:"required,dive"`
}

type SubmitResponse struct {
	ID        string    `json:"id" validate:"required,uuid"`
	FormID    string    `json:"formId" validate:"required,uuid"`
	CreatedAt time.Time `json:"createdAt" validate:"required,datetime"`
	UpdatedAt time.Time `json:"updatedAt" validate:"required,datetime"`
}

type ResponseJSON struct {
	ID          string    `json:"id" validate:"required,uuid"`
	FormID      string    `json:"formId" validate:"required,uuid"`
	SubmittedBy string    `json:"submittedBy" validate:"required,uuid"`
	CreatedAt   time.Time `json:"createdAt" validate:"required,datetime"`
	UpdatedAt   time.Time `json:"updatedAt" validate:"required,datetime"`
}

type GetResponse struct {
	ID        string           `json:"id" validate:"required,uuid"`
	FormID    string           `json:"formId" validate:"required,uuid"`
	Answers   []AnswerResponse `json:"answers" validate:"required,dive"`
	CreatedAt time.Time        `json:"createdAt" validate:"required,datetime"`
	UpdatedAt time.Time        `json:"updatedAt" validate:"required,datetime"`
}

type Store interface {
	Submit(ctx context.Context, formID uuid.UUID, userID uuid.UUID, answers []Answer) (Response, error)
	Get(ctx context.Context, formID uuid.UUID, responseID uuid.UUID) (Response, []Answer, error)
	ListByFormID(ctx context.Context, formID uuid.UUID) ([]Response, error)
	Delete(ctx context.Context, responseID uuid.UUID) error
	GetAnswersByQuestionID(ctx context.Context, questionID uuid.UUID, formID uuid.UUID) ([]GetAnswersByQuestionIDRow, error)
}

type Handler struct {
	logger        *zap.Logger
	validator     *validator.Validate
	problemWriter *problem.HttpWriter
	store         Store
	tracer        trace.Tracer
}

func NewHandler(logger *zap.Logger, validator *validator.Validate, problemWriter *problem.HttpWriter, store Store) *Handler {
	return &Handler{
		logger:        logger,
		validator:     validator,
		problemWriter: problemWriter,
		store:         store,
		tracer:        otel.Tracer("response/handler"),
	}
}

// SubmitHandler submits a response to a form
func (h *Handler) SubmitHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "SubmitResponse")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	formIdStr := r.PathValue("formId")
	formId, err := internal.ParseUUID(formIdStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	var request SubmitRequest
	err = handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &request)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	currentUser, ok := user.GetFromContext(traceCtx)
	if !ok {
		h.problemWriter.WriteError(traceCtx, w, internal.ErrNoUserInContext, logger)
		return
	}

	answers := make([]Answer, len(request.Answers))
	for i, answer := range request.Answers {
		answers[i] = Answer{
			QuestionID: uuid.MustParse(answer.QuestionID),
			Value:      answer.Value,
		}
	}

	newResponse, err := h.store.Submit(traceCtx, formId, currentUser.ID, answers)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	submitResponse := SubmitResponse{
		ID:        newResponse.ID.String(),
		FormID:    newResponse.FormID.String(),
		CreatedAt: newResponse.CreatedAt.Time,
		UpdatedAt: newResponse.UpdatedAt.Time,
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, submitResponse)
}

// ListHandler lists all responses for a form
func (h *Handler) ListHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "ListResponses")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	formIdStr := r.PathValue("formId")
	formId, err := internal.ParseUUID(formIdStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	responses, err := h.store.ListByFormID(traceCtx, formId)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	responseJsons := make([]ResponseJSON, len(responses))
	for i, currentResponse := range responses {
		responseJsons[i] = ResponseJSON{
			ID:          currentResponse.ID.String(),
			FormID:      currentResponse.FormID.String(),
			SubmittedBy: currentResponse.SubmittedBy.String(),
			CreatedAt:   currentResponse.CreatedAt.Time,
			UpdatedAt:   currentResponse.UpdatedAt.Time,
		}
	}
	handlerutil.WriteJSONResponse(w, http.StatusOK, responseJsons)
}

// GetHandler retrieves a response by id
func (h *Handler) GetHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetResponse")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	formIdStr := r.PathValue("formId")
	formId, err := internal.ParseUUID(formIdStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	responseIdStr := r.PathValue("responseId")
	responseId, err := internal.ParseUUID(responseIdStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	currentResponse, answers, err := h.store.Get(traceCtx, formId, responseId)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	answerResponses := make([]AnswerResponse, len(answers))
	for i, answer := range answers {
		answerResponses[i] = AnswerResponse{
			QuestionID: answer.QuestionID.String(),
			Value:      answer.Value,
			UpdatedAt:  answer.UpdatedAt.Time,
		}
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, GetResponse{
		ID:        currentResponse.ID.String(),
		FormID:    currentResponse.FormID.String(),
		CreatedAt: currentResponse.CreatedAt.Time,
		UpdatedAt: currentResponse.UpdatedAt.Time,
		Answers:   answerResponses,
	})
}

// DeleteHandler deletes a response by id
func (h *Handler) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "DeleteResponse")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	responseIdStr := r.PathValue("responseId")
	responseId, err := internal.ParseUUID(responseIdStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	err = h.store.Delete(traceCtx, responseId)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, nil)
}

// GetAnswersByQuestionIDHandler gets answers by question id
func (h *Handler) GetAnswersByQuestionIDHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetAnswersByQuestionID")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	formIdStr := r.PathValue("formId")
	formId, err := internal.ParseUUID(formIdStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	questionIdStr := r.PathValue("questionId")
	questionId, err := internal.ParseUUID(questionIdStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	answers, err := h.store.GetAnswersByQuestionID(traceCtx, questionId, formId)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	answerResponses := make([]AnswerResponse, len(answers))
	for i, answer := range answers {
		answerResponses[i] = AnswerResponse{
			ID:         answer.ID.String(),
			ResponseID: answer.ResponseID.String(),
			QuestionID: answer.QuestionID.String(),
			Type:       question.QuestionType(answer.Type),
			Value:      answer.Value,
			CreatedAt:  answer.CreatedAt.Time,
			UpdatedAt:  answer.UpdatedAt.Time,
		}
	}
	handlerutil.WriteJSONResponse(w, http.StatusOK, answerResponses)
}
