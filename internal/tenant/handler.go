package tenant

import (
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

type Store interface {
	GetSlugStatusWithHistory(ctx context.Context, slug string) (bool, uuid.UUID, []GetSlugHistoryRow, error)
	GetSlugStatus(ctx context.Context, slug string) (bool, uuid.UUID, error)
}

type Handler struct {
	logger        *zap.Logger
	tracer        trace.Tracer
	validator     *validator.Validate
	problemWriter *problem.HttpWriter
	store         Store
}

func NewHandler(
	logger *zap.Logger,
	validator *validator.Validate,
	problemWriter *problem.HttpWriter,
	store Store,
) *Handler {
	return &Handler{
		logger:        logger,
		validator:     validator,
		problemWriter: problemWriter,
		store:         store,
		tracer:        otel.Tracer("tenant/handler"),
	}
}

type ResponseStatus struct {
	Available bool    `json:"available"`
	OrgId     *string `json:"orgId"`
}
type ResponseHistory struct {
	OrgId     string  `json:"orgId"`
	OrgName   string  `json:"orgName"`
	CreatedAt string  `json:"createdAt"`
	EndedAt   *string `json:"endedAt"`
}
type Response struct {
	ResponseStatus `json:"current"`
	History        []ResponseHistory `json:"history"`
}

func (h *Handler) ToResponseHistory(history []GetSlugHistoryRow) []ResponseHistory {
	var responseHistory []ResponseHistory
	for _, h := range history {
		rh := ResponseHistory{
			OrgId:     h.OrgID.String(),
			OrgName:   h.Name.String,
			CreatedAt: h.CreatedAt.Time.Format(time.RFC3339),
		}
		if h.EndedAt.Valid {
			s := h.EndedAt.Time.Format(time.RFC3339)
			rh.EndedAt = &s
		} else {
			rh.EndedAt = nil
		}
		responseHistory = append(responseHistory, rh)
	}
	return responseHistory
}

func (h *Handler) GetStatusWithHistory(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetStatusWithHistory")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	slug := r.PathValue("slug")
	if slug == "" {
		h.logger.Error("User slug is empty", zap.String("path", r.URL.Path))
		problem.New().WriteError(traceCtx, w, handlerutil.ErrInternalServer, h.logger)
		return
	}
	available, orgID, history, err := h.store.GetSlugStatusWithHistory(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, h.logger)
		return
	}

	var orgIDPtr *string
	if orgID != uuid.Nil {
		s := orgID.String()
		orgIDPtr = &s
	}

	response := Response{
		ResponseStatus{
			Available: available,
			OrgId:     orgIDPtr,
		},
		h.ToResponseHistory(history),
	}
	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}

func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetStatus")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	slug := r.PathValue("slug")
	if slug == "" {
		h.logger.Error("User slug is empty", zap.String("path", r.URL.Path))
		problem.New().WriteError(traceCtx, w, handlerutil.ErrInternalServer, h.logger)
		return
	}
	available, orgID, err := h.store.GetSlugStatus(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, h.logger)
		return
	}

	var orgIDPtr *string
	if orgID != uuid.Nil {
		s := orgID.String()
		orgIDPtr = &s
	}

	response := ResponseStatus{
		Available: available,
		OrgId:     orgIDPtr,
	}
	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}
