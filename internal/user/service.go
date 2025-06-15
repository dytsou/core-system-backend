package user

import (
	"context"
	"database/sql"

	databaseutil "github.com/NYCU-SDC/summer/pkg/database"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Service struct {
	logger  *zap.Logger
	queries *Queries
	tracer  trace.Tracer
}

func NewService(logger *zap.Logger, queries *Queries, tracer trace.Tracer) *Service {
	return &Service{
		logger:  logger,
		queries: queries,
		tracer:  otel.Tracer("user/service"),
	}
}

func (s *Service) Create(ctx context.Context, name, username, avatarUrl, role string) (User, error) {
	traceCtx, span := s.tracer.Start(ctx, "user.CreateUser")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)
	avatarUrl = resolveAvatarUrl(name, avatarUrl)

	params := CreateUserParams{
		Name:      name,
		Username:  username,
		AvatarUrl: avatarUrl,
		Role:      role,
	}

	user, err := s.queries.CreateUser(ctx, params)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "create user")
		span.RecordError(err)
		return User{}, err
	}
	return user, nil
}

func (s *Service) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	traceCtx, span := s.tracer.Start(ctx, "user.GetUserByID")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	user, err := s.queries.GetUserByID(ctx, id)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "get user by id")
		span.RecordError(err)
		return User{}, err
	}
	return user, nil
}

func (s *Service) GetUserByUsername(ctx context.Context, username string) (User, error) {
	traceCtx, span := s.tracer.Start(ctx, "user.GetUserByUsername")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	user, err := s.queries.GetUserByUsername(ctx, username)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "get user by username")
		span.RecordError(err)
		return User{}, err
	}
	return user, nil
}

func (s *Service) UpdateUser(ctx context.Context, id uuid.UUID, name, username, avatarUrl, role string) (User, error) {
	traceCtx, span := s.tracer.Start(ctx, "user.UpdateUser")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	avatarUrl = resolveAvatarUrl(name, avatarUrl)

	params := UpdateUserParams{
		ID:        id,
		Name:      name,
		Username:  username,
		AvatarUrl: avatarUrl,
		Role:      role,
	}

	user, err := s.queries.UpdateUser(ctx, params)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "update user")
		span.RecordError(err)
		return User{}, err
	}
	return user, nil
}

func (s *Service) DeleteUser(ctx context.Context, id uuid.UUID) error {
	traceCtx, span := s.tracer.Start(ctx, "user.DeleteUser")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	err := s.queries.DeleteUser(ctx, id)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "delete user")
		span.RecordError(err)
		return err
	}
	return nil
}

func resolveAvatarUrl(name, avatarUrl string) string {
	if avatarUrl == "" {
		return "https://ui-avatars.com/api/?name=" + name
	}
	return avatarUrl
}

func (s *Service) FindOrCreate(ctx context.Context, name, username, avatarUrl, role string) (User, error) {
	traceCtx, span := s.tracer.Start(ctx, "user.FindOrCreate")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	params := FindOrCreateParams{
		ID:        uuid.New(),
		Name:      name,
		Username:  username,
		AvatarUrl: avatarUrl,
		Role:      role,
	}

	user, err := s.queries.FindOrCreate(ctx, params)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "find or create user")
		span.RecordError(err)
		return User{}, err
	}
	return user, nil
}

func (s *Service) ExistsByID(ctx context.Context, id uuid.UUID) (bool, error) {
	traceCtx, span := s.tracer.Start(ctx, "user.ExistsByID")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	_, err := s.queries.GetUserByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		err = databaseutil.WrapDBError(err, logger, "check if user exists by id")
		span.RecordError(err)
		return false, nil
	}
	return true, nil
}
