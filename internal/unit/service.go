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
}

type Wrapper struct {
	Unit Unit
}

func (u Wrapper) GetBase() Base {
	return Base{
		ID:          u.Unit.ID,
		Name:        u.Unit.Name.String,
		Description: u.Unit.Description.String,
		Metadata:    u.Unit.Metadata,
	}
}
func (u Wrapper) SetBase(base Base) {
	u.Unit.ID = base.ID
	u.Unit.Name = pgtype.Text{String: base.Name, Valid: base.Name != ""}
	u.Unit.Description = pgtype.Text{String: base.Description, Valid: base.Description != ""}
	u.Unit.Metadata = base.Metadata
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
	}
}

func (o OrgWrapper) SetBase(base Base) {
	o.Organization.Name = pgtype.Text{String: base.Name, Valid: base.Name != ""}
	o.Organization.Description = pgtype.Text{String: base.Description, Valid: base.Description != ""}
	o.Organization.Metadata = base.Metadata
}

type Querier interface {
	CreateUnit(ctx context.Context, arg CreateUnitParams) (Unit, error)
	CreateUnitWithID(ctx context.Context, arg CreateUnitWithIDParams) (Unit, error)
	CreateOrg(ctx context.Context, arg CreateOrgParams) (Organization, error)
	GetOrgByID(ctx context.Context, id uuid.UUID) (Organization, error)
	GetAllOrganizations(ctx context.Context) ([]Organization, error)
	GetUnitByID(ctx context.Context, id uuid.UUID) (Unit, error)
	GetOrgIDBySlug(ctx context.Context, slug string) (uuid.UUID, error)
	ListSubUnits(ctx context.Context, parentID pgtype.UUID) ([]Unit, error)
	ListOrgSubUnits(ctx context.Context, orgID pgtype.UUID) ([]Unit, error)
	ListSubUnitIDs(ctx context.Context, parentID pgtype.UUID) ([]uuid.UUID, error)
	ListOrgSubUnitIDs(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error)
	UpdateUnit(ctx context.Context, arg UpdateUnitParams) (Unit, error)
	UpdateOrg(ctx context.Context, arg UpdateOrgParams) (Organization, error)
	DeleteOrg(ctx context.Context, id uuid.UUID) error
	DeleteUnit(ctx context.Context, id uuid.UUID) error

	AddParentChild(ctx context.Context, arg AddParentChildParams) (ParentChild, error)
	RemoveParentChild(ctx context.Context, childID uuid.UUID) error
}

type Service struct {
	logger        *zap.Logger
	queries       Querier
	tracer        trace.Tracer
	tenantService *tenant.Service
}

type Base struct {
	ID          uuid.UUID
	Name        string
	Description string
	Metadata    []byte
}

type Type int

const (
	TypeOrg Type = iota
	TypeUnit
)

var typeStrings = [...]string{"organization", "unit"}

func (t Type) String() string {
	if int(t) < 0 || int(t) >= len(typeStrings) {
		return "unknown"
	}
	return typeStrings[t]
}

func NewService(logger *zap.Logger, db DBTX) *Service {
	return &Service{
		logger:        logger,
		queries:       New(db),
		tracer:        otel.Tracer("unit/service"),
		tenantService: tenant.NewService(logger, db),
	}
}

// CreateUnit creates a new unit
func (s *Service) CreateUnit(ctx context.Context, name string, orgID uuid.UUID, description string, metadata []byte) (Unit, error) {
	traceCtx, span := s.tracer.Start(ctx, "CreateUnit")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	unit, err := s.queries.CreateUnit(traceCtx, CreateUnitParams{
		Name:        pgtype.Text{String: name, Valid: name != ""},
		OrgID:       orgID,
		Description: pgtype.Text{String: description, Valid: true},
		Metadata:    metadata,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "create unit")
		span.RecordError(err)
		return Unit{}, err
	}

	_, err = s.queries.AddParentChild(traceCtx, AddParentChildParams{
		ParentID: pgtype.UUID{
			Bytes: orgID,
			Valid: true,
		},
		ChildID: unit.ID,
		OrgID:   orgID,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "add parent-child relationship for created unit")
		span.RecordError(err)
		return Unit{}, err
	}

	logger.Info("Created unit",
		zap.String("unit_id", unit.ID.String()),
		zap.String("org_id", orgID.String()),
		zap.String("name", unit.Name.String),
		zap.String("description", unit.Description.String),
		zap.String("metadata", string(unit.Metadata)))

	return unit, nil
}

func (s *Service) CreateOrg(ctx context.Context, name string, description string, ownerID uuid.UUID, metadata []byte, slug string) (Organization, error) {
	traceCtx, span := s.tracer.Start(ctx, "CreateOrg")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)
	println("OwnerID:", ownerID.String())

	org, err := s.queries.CreateOrg(traceCtx, CreateOrgParams{
		Name:        pgtype.Text{String: name, Valid: name != ""},
		OwnerID:     pgtype.UUID{Bytes: ownerID, Valid: true},
		Description: pgtype.Text{String: description, Valid: true},
		Metadata:    metadata,
		Slug:        slug,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "create organization")
		span.RecordError(err)
		return Organization{}, err
	}

	println("Owner ID", org.OwnerID.String())

	_, err = s.tenantService.Create(traceCtx, org.ID)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "create tenant")
		span.RecordError(err)
		return Organization{}, err
	}

	logger.Info("Created organization",
		zap.String("org_id", org.ID.String()),
		zap.String("org_owner_id", org.OwnerID.String()),
		zap.String("org_name", org.Name.String),
		zap.String("org_slug", org.Slug),
		zap.String("org_description", org.Description.String))

	defaultUnit, err := s.queries.CreateUnitWithID(traceCtx, CreateUnitWithIDParams{
		ID: org.ID,
		Name: pgtype.Text{
			String: "Default Unit",
			Valid:  true,
		},
		OrgID: org.ID,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "create default unit after creating organization")
		span.RecordError(err)
		return Organization{}, err
	}

	_, err = s.queries.AddParentChild(traceCtx, AddParentChildParams{
		ParentID: pgtype.UUID{
			Valid: false,
		},
		ChildID: defaultUnit.ID,
		OrgID:   org.ID,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "add parent-child relationship for default unit")
		span.RecordError(err)
		return Organization{}, err
	}

	logger.Info("Created default unit for organization",
		zap.String("default_unit_id", defaultUnit.ID.String()),
		zap.String("default_unit_org_id", defaultUnit.Description.String),
		zap.String("default_unit_name", defaultUnit.Name.String),
		zap.String("default_unit_description", defaultUnit.Description.String),
		zap.String("default_unit_metadata", string(defaultUnit.Metadata)))

	return org, nil
}

func (s *Service) GetOrgIDBySlug(ctx context.Context, slug string) (uuid.UUID, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetOrgIDBySlug")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	orgID, err := s.queries.GetOrgIDBySlug(traceCtx, slug)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "get organization id by slug")
		span.RecordError(err)
		return uuid.Nil, err
	}

	return orgID, nil
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

	if organizations == nil {
		organizations = []Organization{}
	}

	return organizations, nil
}

// GetByID retrieves a unit by ID
func (s *Service) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID, unitType Type) (GenericUnit, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetByID")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	var returnUnit GenericUnit
	var err error
	switch unitType {
	case TypeOrg:
		var org Organization
		org, err = s.queries.GetOrgByID(traceCtx, orgID)
		returnUnit = OrgWrapper{org}
	case TypeUnit:
		var unit Unit
		unit, err = s.queries.GetUnitByID(traceCtx, id)
		returnUnit = Wrapper{unit}
	default:
		logger.Error("invalid unit type: ", zap.String("unitType", unitType.String()))
		return nil, fmt.Errorf("invalid unit type: %s", unitType.String())
	}

	if err != nil {
		err = databaseutil.WrapDBError(err, logger, fmt.Sprintf("get %s by id", unitType.String()))
		span.RecordError(err)
		return nil, err
	}
	return returnUnit, nil
}

// ListSubUnits retrieves all subunits of a parent unit
func (s *Service) ListSubUnits(ctx context.Context, id uuid.UUID, unitType Type) ([]Unit, error) {
	traceCtx, span := s.tracer.Start(ctx, "ListSubUnits")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	var subUnits []Unit
	var err error
	switch unitType {
	case TypeOrg:
		subUnits, err = s.queries.ListOrgSubUnits(traceCtx, pgtype.UUID{Bytes: id, Valid: true})
	case TypeUnit:
		subUnits, err = s.queries.ListSubUnits(traceCtx, pgtype.UUID{Bytes: id, Valid: true})
	default:
		logger.Error("invalid unit type: ", zap.String("unitType", unitType.String()))
		span.RecordError(err)
		return nil, fmt.Errorf("invalid unit type: %s", unitType.String())
	}

	if err != nil {
		err = databaseutil.WrapDBError(err, logger, fmt.Sprintf("list sub units of an %s", unitType.String()))
		span.RecordError(err)
		return nil, err
	}

	if subUnits == nil {
		subUnits = []Unit{}
	}

	logger.Info(fmt.Sprintf("Listed sub units of an %s", unitType.String()), zap.String("parent_id", id.String()), zap.Int("count", len(subUnits)))
	return subUnits, nil
}

// ListSubUnitIDs retrieves all child unit IDs of a parent unit
func (s *Service) ListSubUnitIDs(ctx context.Context, id uuid.UUID, unitType Type) ([]uuid.UUID, error) {
	traceCtx, span := s.tracer.Start(ctx, "ListSubUnitIDs")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	var subUnitIDs []uuid.UUID
	var err error
	switch unitType {
	case TypeOrg:
		subUnitIDs, err = s.queries.ListOrgSubUnitIDs(traceCtx, id)
	case TypeUnit:
		subUnitIDs, err = s.queries.ListSubUnitIDs(traceCtx, pgtype.UUID{Bytes: id, Valid: true})
	default:
		logger.Error("invalid unit type: ", zap.String("unitType", unitType.String()))
		span.RecordError(err)
		return nil, fmt.Errorf("invalid unit type: %s", unitType.String())
	}

	if err != nil {
		err = databaseutil.WrapDBError(err, logger, fmt.Sprintf("list sub unit IDs of an %s", unitType.String()))
		span.RecordError(err)
		return nil, err
	}

	if subUnitIDs == nil {
		subUnitIDs = []uuid.UUID{}
	}

	logger.Info(fmt.Sprintf("Listed sub unit IDs of an %s", unitType.String()), zap.String("parent_id", id.String()), zap.Int("count", len(subUnitIDs)))
	return subUnitIDs, nil
}

// UpdateUnit updates the base fields of a unit
func (s *Service) UpdateUnit(ctx context.Context, id uuid.UUID, name string, description string, metadata []byte) (Unit, error) {
	traceCtx, span := s.tracer.Start(ctx, "UpdateUnit")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	unit, err := s.queries.UpdateUnit(traceCtx, UpdateUnitParams{
		ID:          id,
		Name:        pgtype.Text{String: name, Valid: name != ""},
		Description: pgtype.Text{String: description, Valid: true},
		Metadata:    metadata,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "update unit")
		span.RecordError(err)
		return Unit{}, err
	}

	logger.Info("Updated unit",
		zap.String("unitID", unit.ID.String()),
		zap.String("unitName", unit.Name.String),
		zap.String("unitDescription", unit.Description.String),
		zap.ByteString("unitMetadata", unit.Metadata),
	)

	return unit, nil
}

// UpdateOrg updates the base fields of an organization
func (s *Service) UpdateOrg(ctx context.Context, id uuid.UUID, name string, description string, metadata []byte, slug string) (Organization, error) {
	traceCtx, span := s.tracer.Start(ctx, "UpdateOrg")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	org, err := s.queries.UpdateOrg(traceCtx, UpdateOrgParams{
		ID:          id,
		Slug:        slug,
		Name:        pgtype.Text{String: name, Valid: name != ""},
		Description: pgtype.Text{String: description, Valid: true},
		Metadata:    metadata,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "update org")
		span.RecordError(err)
		return Organization{}, err
	}

	logger.Info("Updated organization",
		zap.String("orgID", org.ID.String()),
		zap.String("orgName", org.Name.String),
		zap.String("orgSlug", org.Slug),
		zap.String("orgDescription", org.Description.String),
		zap.ByteString("orgMetadata", org.Metadata),
	)

	return org, nil
}

// Delete deletes a unit by ID
func (s *Service) Delete(ctx context.Context, id uuid.UUID, unitType Type) error {
	traceCtx, span := s.tracer.Start(ctx, "Delete")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	switch unitType {
	case TypeOrg:
		err := s.queries.DeleteOrg(traceCtx, id)
		if err != nil {
			err = databaseutil.WrapDBErrorWithKeyValue(err, "organizations", "id", id.String(), logger, "delete organization")
			span.RecordError(err)
			return err
		}
	case TypeUnit:
		// Deletion Check: Ensure not deleting default unit
		unit, err := s.queries.GetUnitByID(traceCtx, id)
		if err != nil {
			err = databaseutil.WrapDBError(err, logger, "get unit by id for deletion check")
			span.RecordError(err)
			return err
		}

		if unit.ID == unit.OrgID {
			err = fmt.Errorf("cannot delete default unit with ID %s", id.String())
			span.RecordError(err)
			logger.Error("Attempted to delete default unit", zap.String("unit_id", id.String()))
			return err
		} else {
			err = s.queries.DeleteUnit(traceCtx, id)
			if err != nil {
				err = databaseutil.WrapDBErrorWithKeyValue(err, "units", "id", id.String(), logger, "delete unit")
				span.RecordError(err)
				return err
			}
		}

	default:
		err := fmt.Errorf("invalid unit type for deletion: %s", unitType)
		span.RecordError(err)
		logger.Error("Invalid unit type for deletion", zap.String("unit_type", unitType.String()))
		return err
	}

	logger.Info(fmt.Sprintf("Deleted %s", unitType), zap.String("ID: ", id.String()))

	return nil
}

// AddParentChild adds a parent-child relationship between two units
func (s *Service) AddParentChild(ctx context.Context, parentID uuid.UUID, childID uuid.UUID, orgID uuid.UUID) (ParentChild, error) {
	traceCtx, span := s.tracer.Start(ctx, "AddParentChild")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	pgParentID := pgtype.UUID{Valid: false}
	if parentID != uuid.Nil {
		pgParentID = pgtype.UUID{Bytes: parentID, Valid: true}
	}

	parentChild, err := s.queries.AddParentChild(traceCtx, AddParentChildParams{
		ParentID: pgParentID,
		ChildID:  childID,
		OrgID:    orgID,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "add parent-child relationship")
		span.RecordError(err)
		return ParentChild{}, err
	}

	logger.Info("Added parent-child relationship", zap.String("parentID", parentID.String()), zap.String("childID", childID.String()))

	return parentChild, nil
}

// RemoveParentChild removes a parent-child relationship between two units
func (s *Service) RemoveParentChild(ctx context.Context, childID uuid.UUID) error {
	traceCtx, span := s.tracer.Start(ctx, "RemoveParentChild")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	err := s.queries.RemoveParentChild(traceCtx, childID)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "remove parent-child relationship")
		span.RecordError(err)
		return err
	}

	logger.Info("Removed parent-child relationship", zap.String("childID", childID.String()))

	return nil
}
