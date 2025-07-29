package form

import (
	"NYCU-SDC/core-system-backend/internal"
	"NYCU-SDC/core-system-backend/internal/form/question"
	"NYCU-SDC/core-system-backend/internal/user"
	"context"
	"net/http"

	handlerutil "github.com/NYCU-SDC/summer/pkg/handler"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/NYCU-SDC/summer/pkg/problem"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Request struct {
	Title       string `json:"title" validate:"required"`
	Description string `json:"description"`
}

type QuestionRequest struct {
	FormID      uuid.UUID
	Required    bool   `json:"required" validate:"required"`
	Type        string `json:"type" validate:"required,oneof=short_text long_text single_choice multiple_choice date"`
	Label       string `json:"label" validate:"required"`
	Description string `json:"description"`
	Order       int32  `json:"order" validate:"required"`
}

type Store interface {
	Create(ctx context.Context, request Request, unitID uuid.UUID, userID uuid.UUID) (Form, error)
	Update(ctx context.Context, id uuid.UUID, request Request, userID uuid.UUID) (Form, error)
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (Form, error)
	List(ctx context.Context) ([]Form, error)
	ListByUnit(ctx context.Context, unitID uuid.UUID) ([]Form, error)
}

type QuestionStore interface {
	Create(ctx context.Context, input question.CreateParams) (question.Question, error)
	Update(ctx context.Context, input question.UpdateParams) (question.Question, error)
	Delete(ctx context.Context, formID uuid.UUID, id uuid.UUID) error
	ListByFormID(ctx context.Context, formID uuid.UUID) ([]question.Question, error)
}

type Handler struct {
	logger *zap.Logger
	tracer trace.Tracer

	validator     *validator.Validate
	problemWriter *problem.HttpWriter

	store         Store
	questionStore QuestionStore
}

func NewHandler(
	logger *zap.Logger,
	validator *validator.Validate,
	problemWriter *problem.HttpWriter,
	store Store,
	questionStore QuestionStore,
) *Handler {
	return &Handler{
		logger:        logger,
		tracer:        otel.Tracer("form/handler"),
		validator:     validator,
		problemWriter: problemWriter,
		store:         store,
		questionStore: questionStore,
	}
}

//func (h *Handler) CreateFormHandler(w http.ResponseWriter, r *http.Request) {
//	traceCtx, span := h.tracer.Start(r.Context(), "CreateFormHandler")
//	defer span.End()
//	logger := logutil.WithContext(traceCtx, h.logger)
//
//	var req Request
//	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req); err != nil {
//		h.problemWriter.WriteError(traceCtx, w, err, logger)
//		return
//	}
//
//	unitIDStr := r.PathValue("unitId")
//	currentUnitID, err := handlerutil.ParseUUID(unitIDStr)
//	if err != nil {
//		h.problemWriter.WriteError(traceCtx, w, err, logger)
//		return
//	}
//
//	currentUser, ok := user.GetFromContext(traceCtx)
//	if !ok {
//		h.problemWriter.WriteError(traceCtx, w, internal.ErrNoUserInContext, logger)
//		return
//	}
//
//	newForm, err := h.store.Create(traceCtx, req, currentUnitID, currentUser.ID)
//	if err != nil {
//		h.problemWriter.WriteError(traceCtx, w, err, logger)
//		return
//	}
//
//	handlerutil.WriteJSONResponse(w, http.StatusCreated, newForm)
//}

func (h *Handler) UpdateFormHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "UpdateFormHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	formIDStr := r.PathValue("id")
	formID, err := handlerutil.ParseUUID(formIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	var req Request
	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req); err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	currentUser, ok := user.GetFromContext(traceCtx)
	if !ok {
		h.problemWriter.WriteError(traceCtx, w, internal.ErrNoUserInContext, logger)
		return
	}

	currentForm, err := h.store.Update(traceCtx, formID, req, currentUser.ID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, currentForm)
}

func (h *Handler) DeleteFormHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "DeleteFormHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	formIDStr := r.PathValue("id")
	formID, err := handlerutil.ParseUUID(formIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	err = h.store.Delete(traceCtx, formID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}
	handlerutil.WriteJSONResponse(w, http.StatusNoContent, nil)
}

func (h *Handler) GetFormHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetFormHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	formIDStr := r.PathValue("id")
	formID, err := handlerutil.ParseUUID(formIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	currentForm, err := h.store.GetByID(traceCtx, formID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}
	handlerutil.WriteJSONResponse(w, http.StatusOK, currentForm)
}

func (h *Handler) ListFormsHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "ListFormHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	forms, err := h.store.List(traceCtx)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}
	handlerutil.WriteJSONResponse(w, http.StatusOK, forms)
}

//func (h *Handler) ListFormsByUnitHandler(w http.ResponseWriter, r *http.Request) {
//	traceCtx, span := h.tracer.Start(r.Context(), "ListFormsByUnitHandler")
//	defer span.End()
//	logger := logutil.WithContext(traceCtx, h.logger)
//
//	unitIDStr := r.PathValue("unitId")
//	unitID, err := handlerutil.ParseUUID(unitIDStr)
//	if err != nil {
//		h.problemWriter.WriteError(traceCtx, w, err, logger)
//		return
//	}
//
//	forms, err := h.store.ListByUnit(traceCtx, unitID)
//	if err != nil {
//		h.problemWriter.WriteError(traceCtx, w, err, logger)
//		return
//	}
//	handlerutil.WriteJSONResponse(w, http.StatusOK, forms)
//}

func (h *Handler) AddQuestionHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "AddQuestionHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	formIDStr := r.PathValue("formId")
	formID, err := handlerutil.ParseUUID(formIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	var req QuestionRequest
	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req); err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	request := question.CreateParams{
		FormID:      formID,
		Required:    req.Required,
		Type:        question.QuestionType(req.Type),
		Label:       pgtype.Text{String: req.Label, Valid: true},
		Description: pgtype.Text{String: req.Description, Valid: true},
		Order:       req.Order,
	}

	createdQuestion, err := h.questionStore.Create(r.Context(), request)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusCreated, createdQuestion)
}

func (h *Handler) UpdateQuestionHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "UpdateQuestionHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	questionIDStr := r.PathValue("questionId")
	questionID, err := handlerutil.ParseUUID(questionIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	formIDStr := r.PathValue("formId")
	formID, err := handlerutil.ParseUUID(formIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	var req QuestionRequest
	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req); err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	request := question.UpdateParams{
		ID:          questionID,
		FormID:      formID,
		Required:    req.Required,
		Type:        question.QuestionType(req.Type),
		Label:       pgtype.Text{String: req.Label, Valid: true},
		Description: pgtype.Text{String: req.Description, Valid: true},
		Order:       req.Order,
	}

	updatedQuestion, err := h.questionStore.Update(traceCtx, request)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, updatedQuestion)

}

func (h *Handler) DeleteQuestionHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "DeleteQuestionHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	formIDStr := r.PathValue("formId")
	formID, err := handlerutil.ParseUUID(formIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	questionIDStr := r.PathValue("questionId")
	questionID, err := handlerutil.ParseUUID(questionIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	err = h.questionStore.Delete(traceCtx, formID, questionID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}
	handlerutil.WriteJSONResponse(w, http.StatusNoContent, nil)
}

func (h *Handler) ListQuestionsHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "ListQuestionsHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	formIDStr := r.PathValue("formId")
	formID, err := handlerutil.ParseUUID(formIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	questions, err := h.questionStore.ListByFormID(traceCtx, formID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}
	handlerutil.WriteJSONResponse(w, http.StatusOK, questions)
}
