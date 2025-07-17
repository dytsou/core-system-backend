package unit

import (
	"NYCU-SDC/core-system-backend/internal/tenant"
	"context"
	"fmt"

	databaseutil "github.com/NYCU-SDC/summer/pkg/database"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type GenericUnit interface {
	GetBase() Base
	SetBase(Base)
	GetType() UnitType
}

type UnitWrapper struct {
	Unit Unit
}

func (u UnitWrapper) GetBase() Base {
	return Base{
		ID:          u.Unit.ID,
		Name:        u.Unit.Name.String,
		Description: u.Unit.Description.String,
		Metadata:    u.Unit.Metadata,
		Type:        u.Unit.Type,
	}
}
func (u UnitWrapper) SetBase(base Base) {
	u.Unit.ID = base.ID
	u.Unit.Name = pgtype.Text{String: base.Name, Valid: base.Name != ""}
	u.Unit.Description = pgtype.Text{String: base.Description, Valid: base.Description != ""}
	u.Unit.Metadata = base.Metadata
	u.Unit.Type = base.Type
}
func (u UnitWrapper) GetType() UnitType {
	return u.Unit.Type
}

type OrgWrapper struct {
	Organization Organization
}

func (o OrgWrapper) GetBase() Base {
	return Base{
		ID:          o.Organization.ID,
		Name:        o.Organization.Name.String,
		Description: o.Organization.Description.String,
		Metadata:    o.Organization.Metadata,
		Type:        o.Organization.Type,
	}
}

func (o OrgWrapper) SetBase(base Base) {
	o.Organization.Name = pgtype.Text{String: base.Name, Valid: base.Name != ""}
	o.Organization.Description = pgtype.Text{String: base.Description, Valid: base.Description != ""}
	o.Organization.Metadata = base.Metadata
}

func (o OrgWrapper) GetType() UnitType {
	return o.Organization.Type
}

type Querier interface {
	CreateUnit(ctx context.Context, arg CreateUnitParams) (Unit, error)
	CreateOrg(ctx context.Context, arg CreateOrgParams) (Organization, error)
	GetOrgByID(ctx context.Context, id uuid.UUID) (Organization, error)
	GetAllOrganizations(ctx context.Context) ([]Organization, error)
	GetUnitByID(ctx context.Context, id uuid.UUID) (Unit, error)
	GetOrgIDBySlug(ctx context.Context, slug string) (uuid.UUID, error)
	ListSubUnits(ctx context.Context, parentID uuid.UUID) ([]Unit, error)
	ListSubUnitIDs(ctx context.Context, parentID uuid.UUID) ([]uuid.UUID, error)
	UpdateUnit(ctx context.Context, arg UpdateUnitParams) (Unit, error)
	UpdateOrg(ctx context.Context, arg UpdateOrgParams) (Organization, error)
	DeleteOrg(ctx context.Context, id uuid.UUID) error
	DeleteUnit(ctx context.Context, id uuid.UUID) error

	AddParentChild(ctx context.Context, arg AddParentChildParams) (ParentChild, error)
	RemoveParentChild(ctx context.Context, arg RemoveParentChildParams) error
	RemoveParentChildByID(ctx context.Context, id uuid.UUID) error
}

// Generic unit for unit and organization

type Service struct {
	logger    *zap.Logger
	queries   Querier
	tracer    trace.Tracer
	tenantSvc *tenant.Service
}

type Base struct {
	ID          uuid.UUID
	Name        string
	Description string
	Metadata    []byte
	Type        UnitType
}

type BaseRequest struct {
	Name        string
	Description string
	Metadata    []byte
}

func NewService(logger *zap.Logger, db DBTX) *Service {
	return &Service{
		logger:    logger,
		queries:   New(db),
		tracer:    otel.Tracer("unit/service"),
		tenantSvc: tenant.NewService(logger, db),
	}
}

// CreateUnit creates a new unit
func (s *Service) CreateUnit(ctx context.Context, Name string, OrgID uuid.UUID, Description string, Metadata []byte) (Unit, error) {
	traceCtx, span := s.tracer.Start(ctx, "CreateUnit")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	unit, err := s.queries.CreateUnit(traceCtx, CreateUnitParams{
		Name:        pgtype.Text{String: Name, Valid: true},
		OrgID:       OrgID,
		Description: pgtype.Text{String: Description, Valid: Description != ""},
		Metadata:    Metadata,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "create unit")
		span.RecordError(err)
		return Unit{}, err
	}

	logger.Debug("Created unit",
		zap.String("unit_id", unit.ID.String()),
		zap.String("org_id", OrgID.String()),
		zap.String("name", unit.Name.String),
		zap.String("description", unit.Description.String),
		zap.String("metadata", string(unit.Metadata)))

	return unit, nil
}

func (s *Service) CreateOrg(ctx context.Context, Name string, Description string, OwnerID uuid.UUID, Metadata []byte, Slug string) (Organization, error) {
	traceCtx, span := s.tracer.Start(ctx, "CreateOrg")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	org, err := s.queries.CreateOrg(traceCtx, CreateOrgParams{
		Name:        pgtype.Text{String: Name, Valid: true},
		OwnerID:     OwnerID,
		Description: pgtype.Text{String: Description, Valid: Description != ""},
		Metadata:    Metadata,
		Slug:        Slug,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "create organization")
		span.RecordError(err)
		return Organization{}, err
	}

	_, err = s.tenantSvc.Create(traceCtx, org.ID)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "create tenant")
		span.RecordError(err)
		return Organization{}, err
	}

	logger.Debug("Created organization",
		zap.String("org_id", org.ID.String()),
		zap.String("org_owner_id", org.OwnerID.String()),
		zap.String("org_name", org.Name.String),
		zap.String("org_slug", org.Slug),
		zap.String("org_description", org.Description.String))
	return org, nil
}

func (s *Service) GetOrgIDBySlug(ctx context.Context, slug string) (uuid.UUID, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetIDBySlug")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)
	org_id, err := s.queries.GetOrgIDBySlug(traceCtx, slug)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "get organization id by slug")
		span.RecordError(err)
		return uuid.Nil, err
	}
	return org_id, nil
}

func (s *Service) GetAllOrganizations(ctx context.Context) ([]Organization, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetAllOrganizations")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)
	organizations, err := s.queries.GetAllOrganizations(traceCtx)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "get all organizations")
		span.RecordError(err)
		return nil, err
	}
	return organizations, nil
}

// GetByID retrieves a unit by ID
func (s *Service) GetByID(ctx context.Context, id uuid.UUID, org_id uuid.UUID, unit_type UnitType) (GenericUnit, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetByID")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)
	switch unit_type {
	case UnitTypeOrganization:
		org, err := s.queries.GetOrgByID(traceCtx, org_id)
		if err != nil {
			err = databaseutil.WrapDBError(err, logger, "get organization by id")
			span.RecordError(err)
			return nil, err
		}
		return OrgWrapper{org}, nil
	case UnitTypeUnit:
		unit, err := s.queries.GetUnitByID(traceCtx, id)
		if err != nil {
			err = databaseutil.WrapDBError(err, logger, "get unit by id")
			span.RecordError(err)
			return nil, err
		}
		return UnitWrapper{unit}, nil
	}
	logger.Error("invalid unit type")
	return nil, fmt.Errorf("invalid unit type")
}

// ListSubUnits retrieves all sub-units of a parent unit
func (s *Service) ListSubUnits(ctx context.Context, parentID uuid.UUID) ([]Unit, error) {
	traceCtx, span := s.tracer.Start(ctx, "ListSubUnits")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)
	subUnits, err := s.queries.ListSubUnits(traceCtx, parentID)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "list sub units")
		span.RecordError(err)
		return nil, err
	}
	logger.Debug("Listed sub units", zap.String("parent_id", parentID.String()), zap.Int("count", len(subUnits)))
	return subUnits, nil
}

// ListSubUnitIDs retrieves all child unit IDs of a parent unit
func (s *Service) ListSubUnitIDs(ctx context.Context, parentID uuid.UUID) ([]uuid.UUID, error) {
	traceCtx, span := s.tracer.Start(ctx, "ListSubUnitIDs")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)
	unitIDs, err := s.queries.ListSubUnitIDs(traceCtx, parentID)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "list sub unit IDs")
		span.RecordError(err)
		return nil, err
	}
	logger.Debug("Listed sub unit IDs", zap.String("parent_id", parentID.String()), zap.Int("count", len(unitIDs)))
	return unitIDs, nil
}

// UpdateUnit updates the base fields of a unit
func (s *Service) UpdateUnit(ctx context.Context, id uuid.UUID, args UpdateUnitParams) (Unit, error) {
	traceCtx, span := s.tracer.Start(ctx, "UpdateUnit")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	unit, err := s.queries.UpdateUnit(traceCtx, UpdateUnitParams{
		ID:          id,
		Name:        args.Name,
		Description: args.Description,
		Metadata:    args.Metadata,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "update unit")
		span.RecordError(err)
		return Unit{}, err
	}
	logger.Debug("Updated unit", zap.String("unit_id", id.String()))
	return unit, nil
}

// UpdateOrg updates the base fields of an organization
func (s *Service) UpdateOrg(ctx context.Context, id uuid.UUID, args UpdateOrgParams) (Organization, error) {
	traceCtx, span := s.tracer.Start(ctx, "UpdateOrg")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	org, err := s.queries.UpdateOrg(traceCtx, UpdateOrgParams{
		ID:          id,
		Name:        args.Name,
		Description: args.Description,
		Metadata:    args.Metadata,
		Slug:        args.Slug,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "update org")
		span.RecordError(err)
		return Organization{}, err
	}
	logger.Debug("Updated org", zap.String("org_id", id.String()))
	return org, nil
}

// Delete deletes a unit by ID
func (s *Service) Delete(ctx context.Context, id uuid.UUID, unitType UnitType) error {
	traceCtx, span := s.tracer.Start(ctx, "Delete")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	switch unitType {
	case UnitTypeUnit:
		err := s.queries.DeleteUnit(traceCtx, id)
		if err != nil {
			err = databaseutil.WrapDBError(err, logger, "delete unit")
			span.RecordError(err)
			return err
		}
	case UnitTypeOrganization:
		err := s.queries.DeleteOrg(traceCtx, id)
		if err != nil {
			err = databaseutil.WrapDBError(err, logger, "delete organization")
			span.RecordError(err)
			return err
		}
	default:
		err := fmt.Errorf("invalid unit type for deletion: %s", unitType)
		span.RecordError(err)
		logger.Error("Invalid unit type for deletion", zap.String("unit_type", string(unitType)))
	}
	logger.Debug("Deleted unit", zap.String("unit_id", id.String()))
	return nil
}

// AddParentChild adds a parent-child relationship between two units
func (s *Service) AddParentChild(ctx context.Context, arg AddParentChildParams) (ParentChild, error) {
	traceCtx, span := s.tracer.Start(ctx, "AddParentChild")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	parentChild, err := s.queries.AddParentChild(traceCtx, arg)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "add parent-child relationship")
		span.RecordError(err)
		return ParentChild{}, err
	}
	logger.Debug("Added parent-child relationship", zap.String("parent_id", arg.ParentID.String()), zap.String("child_id", arg.ChildID.String()))
	return parentChild, nil
}

// RemoveParentChild removes a parent-child relationship between two units
func (s *Service) RemoveParentChild(ctx context.Context, arg RemoveParentChildParams) error {
	traceCtx, span := s.tracer.Start(ctx, "RemoveParentChild")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	err := s.queries.RemoveParentChild(traceCtx, arg)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "remove parent-child relationship")
		span.RecordError(err)
		return err
	}
	logger.Debug("Removed parent-child relationship", zap.String("parent_id", arg.ParentID.String()), zap.String("child_id", arg.ChildID.String()))
	return nil
}

func (s *Service) RemoveParentChildByID(ctx context.Context, id uuid.UUID) error {
	traceCtx, span := s.tracer.Start(ctx, "RemoveParentChildByID")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	err := s.queries.RemoveParentChildByID(traceCtx, id)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "remove parent-child relationship by ID")
		span.RecordError(err)
		return err
	}
	logger.Debug("Removed parent-child relationship by ID", zap.String("id", id.String()))
	return nil
}
