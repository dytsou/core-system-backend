package unit

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

type GenericMember interface {
	GetMember() uuid.UUID
}

type MemberWrapper struct {
	UnitMember UnitMember
}

func (m MemberWrapper) GetMember() uuid.UUID {
	return m.UnitMember.MemberID
}

type OrgMemberWrapper struct {
	OrgMember OrgMember
}

func (o OrgMemberWrapper) GetMember() uuid.UUID {
	return o.OrgMember.MemberID
}
