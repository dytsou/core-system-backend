package tenant

import (
	"NYCU-SDC/core-system-backend/internal"
	"context"
	handlerutil "github.com/NYCU-SDC/summer/pkg/handler"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/NYCU-SDC/summer/pkg/problem"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"net/http"
	"time"
)

type Store interface {
	GetStatusWithHistory(ctx context.Context, slug string) (bool, pgtype.UUID, []GetSlugHistoryRow, error)
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
	Available bool   `json:"available"`
	OrgId     string `json:"orgId"`
}
type ResponseHistory struct {
	Slug      string `json:"slug"`
	OrgId     string `json:"orgId"`
	OrgName   string `json:"orgName"`
	CreatedAt string `json:"createdAt"`
	EndedAt   string `json:"endedAt,omitempty"`
}
type Response struct {
	ResponseStatus `json:"current"`
	History        []ResponseHistory `json:"history"`
}

func (h *Handler) ToResponseHistory(history []GetSlugHistoryRow) []ResponseHistory {
	var responseHistory []ResponseHistory
	for _, h := range history {
		rh := ResponseHistory{
			Slug:      h.Slug,
			OrgId:     h.OrgID.String(),
			OrgName:   h.Name.String,
			CreatedAt: h.CreatedAt.Time.Format(time.RFC3339),
		}
		if h.EndedAt.Valid {
			rh.EndedAt = h.EndedAt.Time.Format(time.RFC3339)
		}
		responseHistory = append(responseHistory, rh)
	}
	return responseHistory
}

func (h *Handler) GetStatusWithHistory(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetStatusWithHistory")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	slug := traceCtx.Value(internal.OrgSlugContextKey).(string)
	available, orgID, history, err := h.store.GetStatusWithHistory(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, h.logger)
		return
	}
	response := Response{
		ResponseStatus{
			Available: available,
			OrgId:     orgID.String(),
		},
		h.ToResponseHistory(history),
	}
	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}

func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetStatus")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	slug := traceCtx.Value(internal.OrgSlugContextKey).(string)
	available, orgID, err := h.store.GetSlugStatus(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, h.logger)
		return
	}
	response := ResponseStatus{
		Available: available,
		OrgId:     orgID.String(),
	}
	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}
