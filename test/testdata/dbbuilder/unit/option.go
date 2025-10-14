package unitbuilder

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Option func(*FactoryParams)

type FactoryParams struct {
	Name        string
	Description string
	OrgID       pgtype.UUID
	Slug        string
	Metadata    []byte
	ParentIDs   []uuid.UUID
	ChildIDs    []uuid.UUID
	OwnerID     pgtype.UUID
}

func WithName(name string) Option {
	return func(p *FactoryParams) {
		p.Name = name
	}
}

func WithDescription(description string) Option {
	return func(p *FactoryParams) {
		p.Description = description
	}
}

func WithOrgID(orgID uuid.UUID) Option {
	return func(p *FactoryParams) {
		p.OrgID = pgtype.UUID{Bytes: orgID, Valid: true}
	}
}

func WithoutOrg() Option {
	return func(p *FactoryParams) {
		p.OrgID = pgtype.UUID{}
	}
}

func WithMetadata(metadata []byte) Option {
	return func(p *FactoryParams) {
		p.Metadata = metadata
	}
}

func WithParent(parentID uuid.UUID) Option {
	return func(p *FactoryParams) {
		p.ParentIDs = append(p.ParentIDs, parentID)
		if !p.OrgID.Valid {
			p.OrgID = pgtype.UUID{Bytes: parentID, Valid: true}
		}
	}
}

func WithChild(childID uuid.UUID) Option {
	return func(p *FactoryParams) {
		if childID == uuid.Nil {
			return
		}
		p.ChildIDs = append(p.ChildIDs, childID)
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
