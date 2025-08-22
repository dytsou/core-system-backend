package unit

import (
	"NYCU-SDC/core-system-backend/internal"
	"NYCU-SDC/core-system-backend/internal/form"
	"NYCU-SDC/core-system-backend/internal/user"
	"context"
	"encoding/json"
	"fmt"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
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

type Store interface {
	GetOrgIDBySlug(ctx context.Context, slug string) (uuid.UUID, error)
	CreateUnit(ctx context.Context, name string, orgID uuid.UUID, desc string, metadata []byte) (Unit, error)
	CreateOrg(ctx context.Context, name string, desc string, creatorID uuid.UUID, metadata []byte, slug string) (Organization, error)
	GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID, unitType Type) (GenericUnit, error)
	GetAllOrganizations(ctx context.Context) ([]Organization, error)
	UpdateUnit(ctx context.Context, id uuid.UUID, params UpdateUnitParams) (Unit, error)
	UpdateOrg(ctx context.Context, id uuid.UUID, params UpdateOrgParams) (Organization, error)
	Delete(ctx context.Context, id uuid.UUID, unitType Type) error
	AddParentChild(ctx context.Context, params AddParentChildParams) (ParentChild, error)
	RemoveParentChild(ctx context.Context, childID uuid.UUID) error
	ListSubUnits(ctx context.Context, id uuid.UUID, unitType Type) ([]Unit, error)
	ListSubUnitIDs(ctx context.Context, id uuid.UUID, unitType Type) ([]uuid.UUID, error)
}

type Handler struct {
	logger        *zap.Logger
	tracer        trace.Tracer
	validator     *validator.Validate
	problemWriter *problem.HttpWriter
	store         Store
	formService   *form.Service
}

func NewHandler(
	logger *zap.Logger,
	validator *validator.Validate,
	problemWriter *problem.HttpWriter,
	store Store,
	formService *form.Service,
) *Handler {
	return &Handler{
		logger:        logger,
		validator:     validator,
		problemWriter: problemWriter,
		store:         store,
		formService:   formService,
		tracer:        otel.Tracer("unit/handler"),
	}
}

type OrgRequest struct {
	Name        string            `json:"name" validate:"required"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	Slug        string            `json:"slug" validate:"required"`
}

type Request struct {
	Name        string            `json:"name" validate:"required"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
}

func (h *Handler) CreateUnit(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "CreateUnit")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	var req Request

	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req); err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid request body: %w", err), h.logger)
		return
	}

	metadataBytes, err := json.Marshal(req.Metadata)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to marshal metadata: %w", err), h.logger)
		return
	}

	orgSlug, err := internal.GetSlugFromContext(traceCtx)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org slug from context: %w", err), h.logger)
		return
	}

	orgID, err := h.store.GetOrgIDBySlug(traceCtx, orgSlug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
		return
	}

	createdUnit, err := h.store.CreateUnit(traceCtx, req.Name, orgID, req.Description, metadataBytes)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to create unit: %w", err), h.logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusCreated, createdUnit)
}

func (h *Handler) CreateOrg(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "CreateOrg")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	var req OrgRequest

	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req); err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid request body: %w", err), h.logger)
		return
	}

	metadataBytes, err := json.Marshal(req.Metadata)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to marshal metadata: %w", err), h.logger)
		return
	}

	currentUser, ok := user.GetFromContext(traceCtx)
	if !ok {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("no user found in request context"), h.logger)
		return
	}

	createdOrg, err := h.store.CreateOrg(traceCtx, req.Name, req.Description, currentUser.ID, metadataBytes, req.Slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to create unit: %w", err), h.logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusCreated, createdOrg)
}

func (h *Handler) GetUnitByID(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetUnitByID")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	slug, err := internal.GetSlugFromContext(traceCtx)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org slug from context: %w", err), h.logger)
		return
	}

	idStr := r.PathValue("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid unit ID: %w", err), h.logger)
		return
	}

	orgID, err := h.store.GetOrgIDBySlug(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
		return
	}

	unit, err := h.store.GetByID(traceCtx, id, orgID, TypeUnit)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get unit by ID: %w", err), h.logger)
		return
	}
	handlerutil.WriteJSONResponse(w, http.StatusOK, unit)
}

func (h *Handler) GetOrgByID(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetOrgByID")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	slug, err := internal.GetSlugFromContext(traceCtx)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org slug from context: %w", err), h.logger)
		return
	}

	orgID, err := h.store.GetOrgIDBySlug(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
		return
	}

	unit, err := h.store.GetByID(traceCtx, orgID, orgID, TypeOrg)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get unit by ID: %w", err), h.logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, unit)
}

func (h *Handler) GetAllOrganizations(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetAllOrganizations")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	organizations, err := h.store.GetAllOrganizations(traceCtx)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get all organizations: %w", err), h.logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, organizations)
}

func (h *Handler) UpdateUnit(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "UpdateUnit")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	var req Request
	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req); err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid request body: %w", err), h.logger)
		return
	}

	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid unit ID: %w", err), h.logger)
		return
	}

	metadataBytes, err := json.Marshal(req.Metadata)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to marshal metadata: %w", err), h.logger)
		return
	}

	params := UpdateUnitParams{
		ID:          id,
		Name:        pgtype.Text{String: req.Name, Valid: true},
		Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Metadata:    metadataBytes,
	}

	updatedUnit, err := h.store.UpdateUnit(traceCtx, id, params)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to update unit: %w", err), h.logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, updatedUnit)
}

func (h *Handler) UpdateOrg(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "UpdateOrg")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	var req OrgRequest
	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req); err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid request body: %w", err), h.logger)
		return
	}

	slug, err := internal.GetSlugFromContext(traceCtx)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org slug from context: %w", err), h.logger)
		return
	}

	id, err := h.store.GetOrgIDBySlug(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
		return
	}

	metadataBytes, err := json.Marshal(req.Metadata)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to marshal metadata: %w", err), h.logger)
		return
	}

	params := UpdateOrgParams{
		ID:          id,
		Name:        pgtype.Text{String: req.Name, Valid: true},
		Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Metadata:    metadataBytes,
		Slug:        req.Slug,
	}

	updatedOrg, err := h.store.UpdateOrg(traceCtx, id, params)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to update organization: %w", err), h.logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, updatedOrg)
}

func (h *Handler) DeleteOrg(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "DeleteOrg")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	slug, err := internal.GetSlugFromContext(traceCtx)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org slug from context: %w", err), h.logger)
		return
	}

	id, err := h.store.GetOrgIDBySlug(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
		return
	}

	err = h.store.Delete(traceCtx, id, TypeOrg)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to delete unit: %w", err), h.logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusNoContent, nil)
}

// DeleteUnit deletes a unit by its ID
func (h *Handler) DeleteUnit(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "DeleteUnit")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid unit ID: %w", err), h.logger)
		return
	}

	err = h.store.Delete(traceCtx, id, TypeUnit)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to delete unit: %w", err), h.logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusNoContent, nil)
}

func (h *Handler) AddParentChild(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "AddParentChild")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	var params AddParentChildParams
	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &params); err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid request body: %w", err), h.logger)
		return
	}

	pc, err := h.store.AddParentChild(traceCtx, params)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to add parent-child relationship: %w", err), h.logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusCreated, pc)
}

func (h *Handler) RemoveParentChild(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "RemoveParentChild")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	cIDStr := r.PathValue("child_id")
	if cIDStr == "" {
		http.Error(w, "parent or child ID not provided", http.StatusBadRequest)
		return
	}

	cID, err := uuid.Parse(cIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid child ID: %w", err), h.logger)
		return
	}

	err = h.store.RemoveParentChild(traceCtx, cID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to remove parent-child relationship: %w", err), h.logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusNoContent, nil)
}

func (h *Handler) ListOrgSubUnits(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "ListOrgSubUnits")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	prefix := "/api/orgs/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	slug, err := internal.GetSlugFromContext(traceCtx)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org slug from context: %w", err), h.logger)
		return
	}

	orgID, err := h.store.GetOrgIDBySlug(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
		return
	}

	subUnits, err := h.store.ListSubUnits(traceCtx, orgID, TypeOrg)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to list sub-units: %w", err), h.logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, subUnits)
}

func (h *Handler) ListUnitSubUnits(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "ListUnitSubUnits")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid unit ID: %w", err), h.logger)
		return
	}
	subUnits, err := h.store.ListSubUnits(traceCtx, id, TypeUnit)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to list sub-units: %w", err), h.logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, subUnits)
}

func (h *Handler) ListOrgSubUnitIDs(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "ListOrgSubUnits")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	slug, err := internal.GetSlugFromContext(traceCtx)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org slug from context: %w", err), h.logger)
		return
	}

	orgID, err := h.store.GetOrgIDBySlug(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
		return
	}

	subUnits, err := h.store.ListSubUnitIDs(traceCtx, orgID, TypeOrg)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to list sub-units: %w", err), h.logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, subUnits)
}

func (h *Handler) ListUnitSubUnitIDs(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "ListUnitSubUnits")
	defer span.End()
	h.logger = logutil.WithContext(traceCtx, h.logger)

	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid unit ID: %w", err), h.logger)
		return
	}

	subUnits, err := h.store.ListSubUnitIDs(traceCtx, id, TypeUnit)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to list sub-units: %w", err), h.logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, subUnits)
}

func (h *Handler) CreateFormUnderUnit(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "CreateFormHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	var req form.Request
	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req); err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	unitIDStr := r.PathValue("unitId")
	currentUnitID, err := handlerutil.ParseUUID(unitIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	currentUser, ok := user.GetFromContext(traceCtx)
	if !ok {
		h.problemWriter.WriteError(traceCtx, w, internal.ErrNoUserInContext, logger)
		return
	}

	newForm, err := h.formService.Create(traceCtx, req, currentUnitID, currentUser.ID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusCreated, newForm)
}

func (h *Handler) ListFormsByUnit(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "ListFormsByUnitHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	unitIDStr := r.PathValue("unitId")
	unitID, err := handlerutil.ParseUUID(unitIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	forms, err := h.formService.ListByUnit(traceCtx, unitID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, forms)
}
