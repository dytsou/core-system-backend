package example

import (
	"NYCU-SDC/core-system-backend/internal"
	"context"
	handlerutil "github.com/NYCU-SDC/summer/pkg/handler"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/NYCU-SDC/summer/pkg/problem"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"net/http"
	"time"
)

type CreateRequest struct {
	Name string `json:"name" validate:"required,max=255"`
}

type UpdateRequest struct {
	Name string `json:"name" validate:"required,max=255"`
}

type Response struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

//go:generate mockery --name=Store
type Store interface {
	GetAll(ctx context.Context) ([]Scoreboard, error)
	GetByID(ctx context.Context, id uuid.UUID) (Scoreboard, error)
	Create(ctx context.Context, req CreateRequest) (Scoreboard, error)
	Update(ctx context.Context, id uuid.UUID, r UpdateRequest) (Scoreboard, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type Handler struct {
	logger        *zap.Logger
	validator     *validator.Validate
	problemWriter *problem.HttpWriter
	tracer        trace.Tracer

	store Store
}

func NewHandler(logger *zap.Logger, validator *validator.Validate, problemWriter *problem.HttpWriter, store Store) *Handler {
	return &Handler{
		validator:     validator,
		logger:        logger,
		tracer:        otel.Tracer("example/handler"),
		problemWriter: problemWriter,
		store:         store,
	}
}

func (h Handler) GetAllHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetAllHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	scoreboards, err := h.store.GetAll(r.Context())
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	response := make([]Response, len(scoreboards))
	for index, scoreboard := range scoreboards {
		response[index] = GenerateResponse(scoreboard)
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}

func (h Handler) GetHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	pathID := r.PathValue("id")
	scoreboardID, err := internal.ParseUUID(pathID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	scoreboard, err := h.store.GetByID(traceCtx, scoreboardID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	response := GenerateResponse(scoreboard)
	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}

func (h Handler) CreateHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "CreateHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	var req CreateRequest
	err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	scoreboard, err := h.store.Create(traceCtx, req)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	logger.Info("Scoreboard created", zap.String("id", scoreboard.ID.String()))

	response := GenerateResponse(scoreboard)
	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}

func (h Handler) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "UpdateHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	pathID := r.PathValue("id")
	scoreboardID, err := internal.ParseUUID(pathID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	var req UpdateRequest
	err = handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	scoreboard, err := h.store.Update(traceCtx, scoreboardID, req)
	if err != nil {
		logger.Error("failed to create scoreboard", zap.Error(err))
	}

	logger.Info("Scoreboard updated", zap.String("id", scoreboard.ID.String()))

	response := GenerateResponse(scoreboard)
	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}

func (h Handler) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "DeleteHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	pathID := r.PathValue("id")
	scoreboardID, err := internal.ParseUUID(pathID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	err = h.store.Delete(traceCtx, scoreboardID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	logger.Info("Scoreboard deleted", zap.String("id", scoreboardID.String()))

	handlerutil.WriteJSONResponse(w, http.StatusNoContent, nil)
}

func GenerateResponse(scoreboard Scoreboard) Response {
	return Response{
		ID:        scoreboard.ID.String(),
		Name:      scoreboard.Name,
		CreatedAt: scoreboard.CreatedAt.Time.Format(time.RFC3339),
		UpdatedAt: scoreboard.UpdatedAt.Time.Format(time.RFC3339),
	}
}
