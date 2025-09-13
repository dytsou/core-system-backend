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
func (s *Service) AddMember(ctx context.Context, unitType Type, id uuid.UUID, memberID uuid.UUID) (GenericMember, error) {
	traceCtx, span := s.tracer.Start(ctx, fmt.Sprintf("Add%sMember", unitType.String()))
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	switch unitType {
	case TypeOrg:
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

	case TypeUnit:
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

	logger.Error("invalid unit type: ", zap.String("unitType", unitType.String()))
	return MemberWrapper{}, fmt.Errorf("invalid unit type: %s", unitType)
}

// ListMembers lists all members of an organization or a unit
func (s *Service) ListMembers(ctx context.Context, unitType Type, id uuid.UUID) ([]uuid.UUID, error) {
	traceCtx, span := s.tracer.Start(ctx, fmt.Sprintf("List%sMembers", unitType.String()))
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	var members []uuid.UUID
	var err error
	switch unitType {
	case TypeOrg:
		members, err = s.queries.ListOrgMembers(traceCtx, id)
	case TypeUnit:
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

	logger.Info(fmt.Sprintf("Listed %s members", unitType.String()),
		zap.String("org_id", id.String()),
		zap.Int("count", len(members)),
		zap.String("members", fmt.Sprintf("%v", members)))

	return members, nil
}

//
//func (s *Service) ListMultiUnitMembers(ctx context.Context, unitIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
//	traceCtx, span := s.tracer.Start(ctx, "ListMultiUnitMembers")
//	defer span.End()
//	logger := logutil.WithContext(traceCtx, s.logger)
//
//	membersMap := make(map[uuid.UUID][]uuid.UUID)
//	if len(unitIDs) == 0 {
//		return membersMap, nil
//	}
//
//	rows, err := s.queries.ListMultiUnitMembers(traceCtx, unitIDs)
//	if err != nil {
//		err = databaseutil.WrapDBError(err, logger, "list multi unit members")
//		span.RecordError(err)
//		return nil, err
//	}
//	defer rows.Close()
//
//	for rows.Next() {
//		var unitID uuid.UUID
//		var memberID uuid.UUID
//		if err := rows.Scan(&unitID, &memberID); err != nil {
//			err = databaseutil.WrapDBError(err, logger, "scan multi unit members")
//			span.RecordError(err)
//			return nil, err
//		}
//		membersMap[unitID] = append(membersMap[unitID], memberID)
//	}
//
//	if err := rows.Err(); err != nil {
//		err = databaseutil.WrapDBError(err, logger, "iterate multi unit members")
//		span.RecordError(err)
//		return nil, err
//	}
//
//	logger.Info("Listed multi unit members",
//		zap.Int("unit_count", len(membersMap)),
//	)
//
//	return membersMap, nil
//}

// RemoveMember removes a member from an organization or a unit
func (s *Service) RemoveMember(ctx context.Context, unitType Type, id uuid.UUID, memberID uuid.UUID) error {
	traceCtx, span := s.tracer.Start(ctx, fmt.Sprintf("Remove%sMember", unitType.String()))
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	var err error
	switch unitType {
	case TypeOrg:
		err = s.queries.RemoveOrgMember(traceCtx, RemoveOrgMemberParams{
			OrgID:    id,
			MemberID: memberID,
		})
	case TypeUnit:
		err = s.queries.RemoveUnitMember(traceCtx, RemoveUnitMemberParams{
			UnitID:   id,
			MemberID: memberID,
		})
	}

	if err != nil {
		err = databaseutil.WrapDBError(err, logger, fmt.Sprintf("remove %s member", unitType.String()))
		span.RecordError(err)
		return err
	}

	logger.Info(fmt.Sprintf("Removed %s member", unitType.String()),
		zap.String("org_id", id.String()),
		zap.String("member_id", memberID.String()))

	return nil
}
