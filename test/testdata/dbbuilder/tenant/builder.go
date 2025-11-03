package tenantbuilder

import (
	"NYCU-SDC/core-system-backend/internal/tenant"
	"NYCU-SDC/core-system-backend/test/testdata/dbbuilder"
	"context"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"testing"
)

type Builder struct {
	t  *testing.T
	db dbbuilder.DBTX
}

func New(t *testing.T, db dbbuilder.DBTX) *Builder {
	return &Builder{t: t, db: db}
}

func (b Builder) Queries() *tenant.Queries {
	return tenant.New(b.db)
}

func (b Builder) Create(opts ...Option) {
	queries := b.Queries()

	p := &FactoryParams{}
	for _, opt := range opts {
		opt(p)
	}

	_, err := queries.Create(context.Background(), tenant.CreateParams{
		ID:         p.ID,
		DbStrategy: tenant.DbStrategyShared,
		OwnerID:    p.OwnerID,
	})
	require.NoError(b.t, err)

	_, err = queries.CreateSlugHistory(context.Background(), tenant.CreateSlugHistoryParams{
		Slug:  p.Slug,
		OrgID: pgtype.UUID{Bytes: p.ID, Valid: true},
	})
	require.NoError(b.t, err)
}
