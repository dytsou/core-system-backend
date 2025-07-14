package unit

import (
	"NYCU-SDC/core-system-backend/internal"
	"NYCU-SDC/core-system-backend/internal/user"
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

type OrgRequest struct {
	Name        string            `json:"name" validate:"required"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	Slug        string            `json:"slug" validate:"required"`
}

type UnitRequest struct {
	Name        string            `json:"name" validate:"required"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
}

func (h *Handler) CreateUnit(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "CreateUnit")
	defer span.End()
	var req UnitRequest

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

	orgSlug, err := internal.GetSlugFromContext(traceCtx)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org slug from context: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	orgID, err := h.service.GetOrgIDBySlug(traceCtx, orgSlug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	createdUnit, err := h.service.CreateUnit(traceCtx, req.Name, orgID, req.Description, metadataBytes)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to create unit: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusCreated, createdUnit)
}

func (h *Handler) CreateOrg(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "CreateOrg")
	defer span.End()
	var req OrgRequest

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

	currentUser, ok := user.GetFromContext(traceCtx)
	if !ok {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("no user found in request context"), h.logger)
		span.RecordError(fmt.Errorf("no user found in request context"))
		return
	}

	fmt.Println("User ID: ", currentUser.ID)

	createdOrg, err := h.service.CreateOrg(traceCtx, req.Name, req.Description, currentUser.ID, metadataBytes, req.Slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to create unit: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusCreated, createdOrg)
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

	unitType := UnitTypeUnit
	orgID, err := h.service.GetOrgIDBySlug(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	unit, err := h.service.GetByID(traceCtx, id, orgID, unitType)
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

	unitType := UnitTypeOrganization
	orgID, err := h.service.GetOrgIDBySlug(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
		span.RecordError(err)
		return
	}
	unit, err := h.service.GetByID(traceCtx, orgID, orgID, unitType)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get unit by ID: %w", err), h.logger)
		span.RecordError(err)
		return
	}
	handlerutil.WriteJSONResponse(w, http.StatusOK, unit)
}

func (h *Handler) UpdateUnit(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "UpdateUnit")
	defer span.End()

	var req UnitRequest
	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req); err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid request body: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/orgs/")
	parts := strings.Split(path, "/")
	// slug := parts[0]
	idStr := parts[2]
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid unit ID: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	metadataBytes, err := json.Marshal(req.Metadata)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to marshal metadata: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	params := UpdateUnitParams{
		ID:          id,
		Name:        pgtype.Text{String: req.Name, Valid: true},
		Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Metadata:    metadataBytes,
	}

	updatedUnit, err := h.service.UpdateUnit(traceCtx, id, params)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to update unit: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	h.logger.Debug("Updated unit",
		zap.String("unit_id", updatedUnit.ID.String()),
		zap.String("unit_name", updatedUnit.Name.String),
		zap.String("unit_description", updatedUnit.Description.String),
		zap.String("unit_type", string(updatedUnit.Type)),
		zap.ByteString("unit_metadata", updatedUnit.Metadata),
	)

	handlerutil.WriteJSONResponse(w, http.StatusOK, updatedUnit)
}

func (h *Handler) UpdateOrg(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "UpdateOrg")
	defer span.End()

	var req OrgRequest
	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req); err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid request body: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/orgs/")
	parts := strings.Split(path, "/")
	slug := parts[0]
	if slug == "" {
		http.Error(w, "slug not provided", http.StatusBadRequest)
		return
	}
	id, err := h.service.GetOrgIDBySlug(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	metadataBytes, err := json.Marshal(req.Metadata)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to marshal metadata: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	params := UpdateOrgParams{
		ID:          id,
		Name:        pgtype.Text{String: req.Name, Valid: true},
		Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Metadata:    metadataBytes,
		Slug:        req.Slug,
	}

	updatedOrg, err := h.service.UpdateOrg(traceCtx, id, params)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to update organization: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	h.logger.Debug("Updated organization",
		zap.String("org_id", updatedOrg.ID.String()),
		zap.String("org_name", updatedOrg.Name.String),
		zap.String("org_description", updatedOrg.Description.String),
		zap.String("unit_type", string(updatedOrg.Type)),
		zap.ByteString("org_metadata", updatedOrg.Metadata),
	)

	handlerutil.WriteJSONResponse(w, http.StatusOK, updatedOrg)
}

func (h *Handler) DeleteOrg(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "DeleteUnit")
	defer span.End()

	path := strings.TrimPrefix(r.URL.Path, "/api/orgs/")
	parts := strings.Split(path, "/")

	slug := parts[0]
	if slug == "" {
		http.Error(w, "slug not provided", http.StatusBadRequest)
		return
	}

	id, err := h.service.GetOrgIDBySlug(traceCtx, slug)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	unitType := UnitTypeOrganization
	err = h.service.Delete(traceCtx, id, unitType)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to delete unit: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	err = h.service.RemoveParentChildByID(traceCtx, id)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to remove parent-child relationships: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	h.logger.Debug("Deleted organization",
		zap.String("org_id", id.String()),
	)

	handlerutil.WriteJSONResponse(w, http.StatusNoContent, nil)
}

// DeleteUnit deletes a unit by its ID
func (h *Handler) DeleteUnit(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "DeleteUnit")
	defer span.End()

	path := strings.TrimPrefix(r.URL.Path, "/api/orgs/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	// slug := parts[0]
	idStr := parts[2]
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid unit ID: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	unitType := UnitTypeUnit
	// org_ID, err := h.service.GetOrgIDBySlug(traceCtx, slug)
	// if err != nil {
	// 	h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to get org ID by slug: %w", err), h.logger)
	// 	span.RecordError(err)
	// 	return
	// }

	err = h.service.Delete(traceCtx, id, unitType)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to delete unit: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	err = h.service.RemoveParentChildByID(traceCtx, id)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to remove parent-child relationships: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	h.logger.Debug("Deleted unit",
		zap.String("unit_id", id.String()),
	)

	handlerutil.WriteJSONResponse(w, http.StatusNoContent, nil)
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

func (h *Handler) RemoveParentChild(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "RemoveParentChild")
	defer span.End()

	prefix := "/api/orgs/relations/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.Split(path, "/")
	p_idStr := parts[1]
	println(p_idStr)
	c_idStr := parts[3]
	println(c_idStr)
	if p_idStr == "" || c_idStr == "" {
		http.Error(w, "parent or child ID not provided", http.StatusBadRequest)
		return
	}
	p_id, err := uuid.Parse(p_idStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid parent ID: %w", err), h.logger)
		span.RecordError(err)
		return
	}
	c_id, err := uuid.Parse(c_idStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid child ID: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	params := RemoveParentChildParams{
		ParentID: p_id,
		ChildID:  c_id,
	}

	err = h.service.RemoveParentChild(traceCtx, params)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to remove parent-child relationship: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	h.logger.Debug("Removed parent-child relationship",
		zap.String("parent_id", params.ParentID.String()),
		zap.String("child_id", params.ChildID.String()),
	)

	handlerutil.WriteJSONResponse(w, http.StatusNoContent, nil)
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

func (h *Handler) ListOrgSubUnitIDs(w http.ResponseWriter, r *http.Request) {
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

	subUnits, err := h.service.ListSubUnitIDs(traceCtx, org_ID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to list sub-units: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, subUnits)
}

func (h *Handler) ListUnitSubUnitIDs(w http.ResponseWriter, r *http.Request) {
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
	subUnits, err := h.service.ListSubUnitIDs(traceCtx, id)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to list sub-units: %w", err), h.logger)
		span.RecordError(err)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, subUnits)
}
