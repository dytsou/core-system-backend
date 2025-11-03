package tenantbuilder

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Option func(*FactoryParams)

type FactoryParams struct {
	ID      uuid.UUID
	Slug    string
	OwnerID pgtype.UUID
}

func WithID(id uuid.UUID) Option {
	return func(p *FactoryParams) {
		p.ID = id
	}
}

func WithOwnerID(ownerID uuid.UUID) Option {
	return func(p *FactoryParams) {
		p.OwnerID = pgtype.UUID{Bytes: ownerID, Valid: true}
	}
}

func WithSlug(slug string) Option {
	return func(p *FactoryParams) {
		p.Slug = slug
	}
}
