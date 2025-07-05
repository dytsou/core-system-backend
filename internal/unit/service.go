package unit

import (
	"context"

	databaseutil "github.com/NYCU-SDC/summer/pkg/database"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Querier interface {
	CreateUnit(ctx context.Context, arg CreateUnitParams) (Unit, error)
	GetUnitByID(ctx context.Context, id uuid.UUID) (Unit, error)
	GetOrgByID(ctx context.Context, id uuid.UUID) (Organization, error)
	ListSubUnits(ctx context.Context, parentID uuid.UUID) ([]Unit, error)
	ListSubUnitIDs(ctx context.Context, parentID uuid.UUID) ([]uuid.UUID, error)
	UpdateUnit(ctx context.Context, arg UpdateUnitParams) (Unit, error)
	DeleteUnit(ctx context.Context, id uuid.UUID) error
	CreateOrg(ctx context.Context, arg CreateOrgParams) (Organization, error)

	AddParentChild(ctx context.Context, pc AddParentChildParams) (ParentChild, error)
	RemoveParentChild(ctx context.Context, parentID, childID uuid.UUID) error
}

type Service struct {
	logger  *zap.Logger
	queries Querier
	tracer  trace.Tracer
}

type Base struct {
	ID          uuid.UUID
	Name        string
	Description string
	Metadata    []byte
	Type        UnitType
}

func NewService(logger *zap.Logger, db DBTX) *Service {
	return &Service{
		logger:  logger,
		queries: New(db),
		tracer:  otel.Tracer("unit/service"),
	}
}

// CreateUnit creates a new unit
func (s *Service) CreateUnit(ctx context.Context, base Base) (Unit, error) {
	traceCtx, span := s.tracer.Start(ctx, "CreateUnit")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	unit, err := s.queries.CreateUnit(traceCtx, CreateUnitParams{
		ID:          base.ID,
		Name:        pgtype.Text{String: base.Name, Valid: base.Name != ""},
		Description: pgtype.Text{String: base.Description, Valid: base.Description != ""},
		Metadata:    base.Metadata,
		Type:        base.Type,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "create unit")
		span.RecordError(err)
		return Unit{}, err
	}
	logger.Debug("Created unit", zap.String("unit_id", unit.ID.String()))
	return unit, nil
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

// GetUnitByID retrieves a unit by ID
func (s *Service) GetUnitByID(ctx context.Context, id uuid.UUID) (Unit, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetUnitByID")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	unit, err := s.queries.GetUnitByID(traceCtx, id)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "get unit by id")
		span.RecordError(err)
		return Unit{}, err
	}
	return unit, nil
}

// UpdateUnit updates the base fields of a unit
func (s *Service) UpdateUnit(ctx context.Context, id uuid.UUID, base Base) (Unit, error) {
	traceCtx, span := s.tracer.Start(ctx, "UpdateUnit")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	unit, err := s.queries.UpdateUnit(traceCtx, UpdateUnitParams{
		ID:          id,
		Name:        pgtype.Text{String: base.Name, Valid: base.Name != ""},
		Description: pgtype.Text{String: base.Description, Valid: base.Description != ""},
		Metadata:    base.Metadata,
		Type:        base.Type,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "update unit")
		span.RecordError(err)
		return Unit{}, err
	}
	logger.Debug("Updated unit", zap.String("unit_id", id.String()))
	return unit, nil
}

// DeleteUnit deletes a unit by ID
func (s *Service) DeleteUnit(ctx context.Context, id uuid.UUID) error {
	traceCtx, span := s.tracer.Start(ctx, "DeleteUnit")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	err := s.queries.DeleteUnit(traceCtx, id)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "delete unit")
		span.RecordError(err)
		return err
	}
	logger.Debug("Deleted unit", zap.String("unit_id", id.String()))
	return nil
}

// ListChildUnits retrieves all child units of a parent unit
func (s *Service) ListChildUnits(ctx context.Context, parentID uuid.UUID) ([]Unit, error) {
	traceCtx, span := s.tracer.Start(ctx, "ListChildUnits")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)
	units, err := s.queries.ListChildUnits(traceCtx, parentID)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "list child units")
		span.RecordError(err)
		return nil, err
	}
	logger.Debug("Listed child units", zap.String("parent_id", parentID.String()), zap.Int("count", len(units)))
	return units, nil
}

// AddParentChild adds a parent-child relationship between two units
func (s *Service) AddParentChild(ctx context.Context, pc AddParentChildParams) (ParentChild, error) {
	traceCtx, span := s.tracer.Start(ctx, "AddParentChild")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	parentChild, err := s.queries.AddParentChild(traceCtx, pc)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "add parent-child relationship")
		span.RecordError(err)
		return ParentChild{}, err
	}
	logger.Debug("Added parent-child relationship", zap.String("parent_id", pc.ParentID.String()), zap.String("child_id", pc.ChildID.String()))
	return parentChild, nil
}

// RemoveParentChild removes a parent-child relationship between two units
func (s *Service) RemoveParentChild(ctx context.Context, parentID, childID uuid.UUID) error {
	traceCtx, span := s.tracer.Start(ctx, "RemoveParentChild")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	err := s.queries.RemoveParentChild(traceCtx, parentID, childID)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "remove parent-child relationship")
		span.RecordError(err)
		return err
	}
	logger.Debug("Removed parent-child relationship", zap.String("parent_id", parentID.String()), zap.String("child_id", childID.String()))
	return nil
}
