package distribute

import (
	"NYCU-SDC/core-system-backend/internal"
	"NYCU-SDC/core-system-backend/internal/user"
	"context"

	databaseutil "github.com/NYCU-SDC/summer/pkg/database"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type UnitStore interface {
	ListMembers(ctx context.Context, id uuid.UUID) ([]user.Profile, error)
	ListUnitsMembers(ctx context.Context, unitIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error)
}

type Service struct {
	logger *zap.Logger
	tracer trace.Tracer
	store  UnitStore
}

func NewService(logger *zap.Logger, store UnitStore) *Service {
	return &Service{
		logger: logger,
		store:  store,
		tracer: otel.Tracer("distribute/service"),
	}
}

// Todo: Need to use SimpleUser instead of uuid.UUID
func (s *Service) GetOrgRecipients(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetOrgRecipients")
	defer span.End()
	logger := internal.WithContext(traceCtx, s.logger)

	recipients, err := s.store.ListMembers(traceCtx, orgID)
	if err != nil {
		return nil, err
	}

	logger.Debug("Organization recipients resolved",
		zap.String("org_id", orgID.String()),
		zap.Int("recipients_count", len(recipients)),
	)

	ids := make([]uuid.UUID, 0, len(recipients))
	for _, r := range recipients {
		ids = append(ids, r.ID)
	}

	return ids, nil
}

func (s *Service) GetRecipients(ctx context.Context, unitIDs []uuid.UUID) ([]uuid.UUID, error) {
	ctx, span := s.tracer.Start(ctx, "GetRecipients")
	defer span.End()
	logger := logutil.WithContext(ctx, s.logger)

	all := make([]uuid.UUID, 0)

	memberMap, err := s.store.ListUnitsMembers(ctx, unitIDs)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "list units members")
		span.RecordError(err)
		return nil, err
	}

	for _, ms := range memberMap {
		all = append(all, ms...)
	}

	seen := make(map[uuid.UUID]struct{}, len(all))
	uniq := make([]uuid.UUID, 0, len(all))
	for _, id := range all {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}

	logger.Debug("Recipients resolved",
		zap.Int("unit_count", len(unitIDs)),
		zap.Int("unique_recipients", len(uniq)),
	)

	return uniq, nil
}
