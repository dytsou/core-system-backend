package distribute

import (
	"context"
	"fmt"
	databaseutil "github.com/NYCU-SDC/summer/pkg/database"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type UnitStore interface {
	UsersByOrg(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error)
	UsersByUnit(ctx context.Context, unitIDs uuid.UUID) ([]uuid.UUID, error)
}

type Distributor interface {
	GetRecipients(ctx context.Context, orgIDs, unitIDs []uuid.UUID) ([]uuid.UUID, error)
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

func (s *Service) GetRecipients(ctx context.Context, orgIDs, unitIDs []uuid.UUID) ([]uuid.UUID, error) {
	ctx, span := s.tracer.Start(ctx, "GetRecipients")
	defer span.End()
	logger := logutil.WithContext(ctx, s.logger)

	all := make([]uuid.UUID, 0, len(orgIDs)+len(unitIDs))

	for _, orgID := range orgIDs {
		ids, err := s.store.UsersByOrg(ctx, orgID)
		if err != nil {
			err = databaseutil.WrapDBError(err, logger, fmt.Sprintf("list org members (org_id=%s)", orgID))
			span.RecordError(err)
			return nil, err
		}
		all = append(all, ids...)
	}

	for _, unitID := range unitIDs {
		ids, err := s.store.UsersByUnit(ctx, unitID)
		if err != nil {
			err = databaseutil.WrapDBError(err, logger, fmt.Sprintf("list unit members (unit_id=%s)", unitID))
			span.RecordError(err)
			return nil, err
		}
		all = append(all, ids...)
	}

	//remove duplicated
	seen := make(map[uuid.UUID]struct{}, len(all))
	uniq := make([]uuid.UUID, 0, len(all))
	for _, id := range all {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}

	logger.Info("Recipients resolved",
		zap.Int("org_count", len(orgIDs)),
		zap.Int("unit_count", len(unitIDs)),
		zap.Int("unique_recipients", len(uniq)),
	)

	return uniq, nil
}
