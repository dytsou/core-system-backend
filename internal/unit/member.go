package unit

import (
	"fmt"
	databaseutil "github.com/NYCU-SDC/summer/pkg/database"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

// AddMember adds a member to an organization or a unit
func (s *Service) AddMember(ctx context.Context, unitType string, id uuid.UUID, memberID uuid.UUID) (GenericMember, error) {
	mapping := map[string]string{
		"unit":         "Unit",
		"organization": "Org",
	}

	traceCtx, span := s.tracer.Start(ctx, fmt.Sprintf("Add%sMember", mapping[unitType]))
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	switch unitType {
	case "organization":
		orgMember, err := s.queries.AddOrgMember(traceCtx, AddOrgMemberParams{
			OrgID:    id,
			MemberID: memberID,
		})
		if err != nil {
			err = databaseutil.WrapDBError(err, logger, "add org member relationship")
			span.RecordError(err)
			return OrgMemberWrapper{}, err
		}

		logger.Info("Added organization member",
			zap.String("org_id", orgMember.OrgID.String()),
			zap.String("member_id", orgMember.MemberID.String()))

		return OrgMemberWrapper{orgMember}, nil

	case "unit":
		unitMember, err := s.queries.AddUnitMember(traceCtx, AddUnitMemberParams{
			UnitID:   id,
			MemberID: memberID,
		})
		if err != nil {
			err = databaseutil.WrapDBError(err, logger, "add unit member relationship")
			span.RecordError(err)
			return MemberWrapper{}, err
		}

		logger.Info("Added unit member",
			zap.String("unit_id", unitMember.UnitID.String()),
			zap.String("member_id", unitMember.MemberID.String()))

		return MemberWrapper{unitMember}, nil
	}

	logger.Error("invalid unit type: ", zap.String("unitType", unitType))
	return MemberWrapper{}, fmt.Errorf("invalid unit type: %s", unitType)
}

// ListMembers lists all members of an organization or a unit
func (s *Service) ListMembers(ctx context.Context, unitType string, id uuid.UUID) ([]uuid.UUID, error) {
	mapping := map[string]string{
		"unit":         "Unit",
		"organization": "Org",
	}

	traceCtx, span := s.tracer.Start(ctx, fmt.Sprintf("List%sMembers", mapping[unitType]))
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	var members []uuid.UUID
	var err error
	switch unitType {
	case "organization":
		members, err = s.queries.ListOrgMembers(traceCtx, id)
	case "unit":
		members, err = s.queries.ListUnitMembers(traceCtx, id)
	}

	if err != nil {
		err = databaseutil.WrapDBError(err, logger, fmt.Sprintf("list %s members", unitType))
		span.RecordError(err)
		return nil, err
	}

	if members == nil {
		members = []uuid.UUID{}
	}

	logger.Info(fmt.Sprintf("Listed %s members", unitType),
		zap.String("org_id", id.String()),
		zap.Int("count", len(members)),
		zap.String("members", fmt.Sprintf("%v", members)))

	return members, nil
}

// RemoveMember removes a member from an organization or a unit
func (s *Service) RemoveMember(ctx context.Context, unitType Type, id uuid.UUID, memberID uuid.UUID) error {
	mapping := map[string]string{
		"unit":         "Unit",
		"organization": "Org",
	}

	traceCtx, span := s.tracer.Start(ctx, fmt.Sprintf("Remove%sMember", mapping[unitType]))
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	var err error
	switch unitType {
	case "organization":
		err = s.queries.RemoveOrgMember(traceCtx, RemoveOrgMemberParams{
			OrgID:    id,
			MemberID: memberID,
		})
	case "unit":
		err = s.queries.RemoveUnitMember(traceCtx, RemoveUnitMemberParams{
			UnitID:   id,
			MemberID: memberID,
		})
	}

	if err != nil {
		err = databaseutil.WrapDBError(err, logger, fmt.Sprintf("remove %s member", unitType))
		span.RecordError(err)
		return err
	}

	logger.Info(fmt.Sprintf("Removed %s member", unitType),
		zap.String("org_id", id.String()),
		zap.String("member_id", memberID.String()))

	return nil
}
