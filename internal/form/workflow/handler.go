package workflow

import (
	"NYCU-SDC/core-system-backend/internal"
	"NYCU-SDC/core-system-backend/internal/user"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	handlerutil "github.com/NYCU-SDC/summer/pkg/handler"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/NYCU-SDC/summer/pkg/problem"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Store interface {
	Get(ctx context.Context, formID uuid.UUID) (GetRow, error)
	Update(ctx context.Context, formID uuid.UUID, workflow []byte, userID uuid.UUID) (UpdateRow, error)
	CreateNode(ctx context.Context, formID uuid.UUID, nodeType NodeType, userID uuid.UUID) (CreateNodeRow, error)
	DeleteNode(ctx context.Context, formID uuid.UUID, nodeID uuid.UUID, userID uuid.UUID) ([]byte, error)
	Activate(ctx context.Context, formID uuid.UUID) (ActivateRow, error)
}

type Handler struct {
	logger *zap.Logger
	tracer trace.Tracer

	validator     *validator.Validate
	problemWriter *problem.HttpWriter

	store Store
}

func NewHandler(
	logger *zap.Logger,
	validator *validator.Validate,
	problemWriter *problem.HttpWriter,
	store Store,
) *Handler {
	return &Handler{
		logger:        logger,
		tracer:        otel.Tracer("workflow/handler"),
		validator:     validator,
		problemWriter: problemWriter,
		store:         store,
	}
}

type createNodeRequest struct {
	Type string `json:"type" validate:"required,oneof=section condition"`
}

type createNodeResponse struct {
	ID    string      `json:"id"`
	Type  string      `json:"type"`
	Label interface{} `json:"label"`
}

func (h *Handler) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetWorkflow")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	formIDStr := r.PathValue("id")
	formID, err := handlerutil.ParseUUID(formIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	row, err := h.store.Get(traceCtx, formID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, json.RawMessage(row.Workflow))
}

func (h *Handler) UpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "UpdateWorkflow")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	formIDStr := r.PathValue("id")
	formID, err := handlerutil.ParseUUID(formIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	currentUser, ok := user.GetFromContext(traceCtx)
	if !ok {
		h.problemWriter.WriteError(traceCtx, w, internal.ErrNoUserInContext, logger)
		return
	}

	var req json.RawMessage
	err = handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	row, err := h.store.Update(traceCtx, formID, []byte(req), currentUser.ID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	workflowResponse, err := json.Marshal(row.Workflow)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, json.RawMessage(workflowResponse))
}

func (h *Handler) CreateNode(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "CreateNode")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	formIDStr := r.PathValue("formId")
	formID, err := handlerutil.ParseUUID(formIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	var req createNodeRequest
	err = handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	currentUser, ok := user.GetFromContext(traceCtx)
	if !ok {
		h.problemWriter.WriteError(traceCtx, w, internal.ErrNoUserInContext, logger)
		return
	}

	nodeType := NodeType(req.Type)
	created, err := h.store.CreateNode(traceCtx, formID, nodeType, currentUser.ID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, createNodeResponse{
		ID:    created.ID.String(),
		Type:  string(created.Type),
		Label: created.Label,
	})
}

func (h *Handler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "DeleteNode")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	formIDStr := r.PathValue("formId")
	formID, err := handlerutil.ParseUUID(formIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	nodeIDStr := r.PathValue("nodeId")
	nodeID, err := handlerutil.ParseUUID(nodeIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	currentUser, ok := user.GetFromContext(traceCtx)
	if !ok {
		h.problemWriter.WriteError(traceCtx, w, internal.ErrNoUserInContext, logger)
		return
	}

	_, err = h.store.DeleteNode(traceCtx, formID, nodeID, currentUser.ID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusNoContent, nil)
}

func (h *Handler) ActivateWorkflow(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "ActivateWorkflow")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	formIDStr := r.PathValue("id")
	formID, err := handlerutil.ParseUUID(formIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	activatedVersion, err := h.store.Activate(traceCtx, formID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	if !activatedVersion.IsActive {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to activate workflow"), logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, nil)
}
