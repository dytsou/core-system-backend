package unit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	handlerutil "github.com/NYCU-SDC/summer/pkg/handler"
	"github.com/NYCU-SDC/summer/pkg/problem"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Handler struct {
	logger        *zap.Logger
	validator     *validator.Validate
	problemWriter *problem.HttpWriter
	service       *Service
	tracer        trace.Tracer
}

func NewHandler(
	logger *zap.Logger,
	validator *validator.Validate,
	problemWriter *problem.HttpWriter,
	service *Service,
) *Handler {
	return &Handler{
		logger:        logger,
		validator:     validator,
		problemWriter: problemWriter,
		service:       service,
		tracer:        otel.Tracer("user/handler"),
	}
}

type CreateOrgRequest struct {
	Name        string            `json:"name" validate:"required"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	Slug        string            `json:"slug" validate:"required"`
}

type CreateUnitRequest struct {
	Name        string            `json:"name" validate:"required"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
}

func (h *Handler) CreateUnit(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "CreateUnit")
	defer span.End()
	var req CreateUnitRequest

	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req); err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid request body: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	metadataBytes, err := json.Marshal(req.Metadata)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to marshal metadata: %w", err), h.logger)
		span.RecordError(err)
		return
	}
	createdUnit, err := h.service.CreateUnit(traceCtx, CreateUnitParams{
		Name:        pgtype.Text{String: req.Name, Valid: true},
		Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Metadata:    metadataBytes,
	})
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to create unit: %w", err), h.logger)
		span.RecordError(err)
		return
	}
	h.logger.Debug("Creat unit",
		zap.String("unit_id", createdUnit.ID.String()),
		zap.String("unit_name", createdUnit.Name.String),
		zap.String("unit_description", createdUnit.Description.String),
		zap.String("unit_type", string(createdUnit.Type)),
		zap.ByteString("unit_metadata", createdUnit.Metadata),
	)
	handlerutil.WriteJSONResponse(w, http.StatusCreated, createdUnit)
}

func (h *Handler) CreateOrg(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "CreateOrg")
	defer span.End()
	var req CreateOrgRequest

	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req); err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid request body: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	metadataBytes, err := json.Marshal(req.Metadata)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to marshal metadata: %w", err), h.logger)
		span.RecordError(err)
		return
	}
	params := CreateOrgParams{
		Name:        pgtype.Text{String: req.Name, Valid: true},
		Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Metadata:    metadataBytes,
		Slug:        req.Slug,
	}

	createdUnit, err := h.service.CreateOrg(traceCtx, params)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to create unit: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	h.logger.Debug("Creat unit",
		zap.String("unit_id", createdUnit.ID.String()),
		zap.String("unit_name", createdUnit.Name.String),
		zap.String("unit_description", createdUnit.Description.String),
		zap.String("unit_type", string(createdUnit.Type)),
		zap.ByteString("unit_metadata", createdUnit.Metadata),
	)
	handlerutil.WriteJSONResponse(w, http.StatusCreated, createdUnit)
}

func (h *Handler) GetUnitByID(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetUnitByID")
	defer span.End()
	path := strings.TrimPrefix(r.URL.Path, "/api/orgs/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	slug := parts[0]
	idStr := parts[2]

	id, err := uuid.Parse(idStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid unit ID: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	var unitType UnitType = UnitTypeUnit
	org_ID, err := h.service.GetOrgIDBySlug(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
		span.RecordError(err)
		return
	}
	unit, err := h.service.GetByID(traceCtx, id, org_ID, unitType)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get unit by ID: %w", err), h.logger)
		span.RecordError(err)
		return
	}
	handlerutil.WriteJSONResponse(w, http.StatusOK, unit)
}

func (h *Handler) GetOrgByID(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetUnitByID")
	defer span.End()

	prefix := "/api/orgs/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	slug := strings.TrimPrefix(r.URL.Path, prefix)
	if slug == "" {
		http.Error(w, "slug not provided", http.StatusBadRequest)
		return
	}

	var unitType UnitType = UnitTypeOrganization
	org_ID, err := h.service.GetOrgIDBySlug(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
		span.RecordError(err)
		return
	}
	unit, err := h.service.GetByID(traceCtx, org_ID, org_ID, unitType)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get unit by ID: %w", err), h.logger)
		span.RecordError(err)
		return
	}
	handlerutil.WriteJSONResponse(w, http.StatusOK, unit)
}

func (h *Handler) AddParentChild(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "AddParentChild")
	defer span.End()

	var params AddParentChildParams
	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &params); err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid request body: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	pc, err := h.service.AddParentChild(traceCtx, params)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to add parent-child relationship: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	h.logger.Debug("Added parent-child relationship",
		zap.String("parent_id", pc.ParentID.String()),
		zap.String("child_id", pc.ChildID.String()),
	)

	handlerutil.WriteJSONResponse(w, http.StatusCreated, pc)
}

func (h *Handler) ListOrgSubUnits(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "ListOrgSubUnits")
	defer span.End()

	prefix := "/api/orgs/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.Split(path, "/")
	slug := parts[0]
	if slug == "" {
		http.Error(w, "slug not provided", http.StatusBadRequest)
		return
	}

	org_ID, err := h.service.GetOrgIDBySlug(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	subUnits, err := h.service.ListSubUnits(traceCtx, org_ID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to list sub-units: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, subUnits)
}

func (h *Handler) ListUnitSubUnits(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "ListUnitSubUnits")
	defer span.End()

	path := strings.TrimPrefix(r.URL.Path, "/api/orgs/")
	parts := strings.Split(path, "/")
	idStr := parts[2]
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid unit ID: %w", err), h.logger)
		span.RecordError(err)
		return
	}
	subUnits, err := h.service.ListSubUnits(traceCtx, id)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to list sub-units: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, subUnits)
}
