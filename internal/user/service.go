package user

import (
	"context"
	"database/sql"
	"errors"

	databaseutil "github.com/NYCU-SDC/summer/pkg/database"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Service struct {
	logger  *zap.Logger
	queries *Queries
	tracer  trace.Tracer
}

func NewService(logger *zap.Logger, db DBTX) *Service {
	return &Service{
		logger:  logger,
		queries: New(db),
		tracer:  otel.Tracer("user/service"),
	}
}

func (s *Service) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetUserByID")
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

func resolveAvatarUrl(name, avatarUrl string) string {
	if avatarUrl == "" {
		return "https://ui-avatars.com/api/?name=" + name
	}
	return avatarUrl
}

func (s *Service) FindOrCreate(ctx context.Context, name, username, avatarUrl string, role []string, oauthProvider, oauthProviderID string) (User, error) {
	traceCtx, span := s.tracer.Start(ctx, "FindOrCreate")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	existingUser, err := s.queries.GetUserByAuth(ctx, GetUserByAuthParams{
		Provider:   oauthProvider,
		ProviderID: oauthProviderID,
	})
	if err == nil {
		// User exists, return it
		logger.Debug("Found existing user", zap.String("provider", oauthProvider), zap.String("provider_id", oauthProviderID))
		return existingUser, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		logger.Debug("User not found, creating new user", zap.String("provider", oauthProvider), zap.String("provider_id", oauthProviderID))
	} else {
		// Unexpected database error
		err = databaseutil.WrapDBError(err, logger, "get user by auth")
		span.RecordError(err)
		return User{}, err
	}

	// User doesn't exist, create new user
	avatarUrl = resolveAvatarUrl(name, avatarUrl)
	if len(role) == 0 {
		role = []string{"user"}
	}

	newUser, err := s.queries.CreateUser(ctx, CreateUserParams{
		Name:      pgtype.Text{String: name, Valid: name != ""},
		Username:  pgtype.Text{String: username, Valid: username != ""},
		AvatarUrl: pgtype.Text{String: avatarUrl, Valid: avatarUrl != ""},
		Role:      role,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "create user")
		span.RecordError(err)
		return User{}, err
	}

	logger.Debug("Created new user", zap.String("user_id", newUser.ID.String()), zap.String("username", newUser.Username.String))

	// Create auth entry
	_, err = s.queries.CreateAuth(ctx, CreateAuthParams{
		UserID:     newUser.ID,
		Provider:   oauthProvider,
		ProviderID: oauthProviderID,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "create auth")
		span.RecordError(err)
		return User{}, err
	}

	logger.Debug("Created auth entry", zap.String("user_id", newUser.ID.String()), zap.String("provider", oauthProvider), zap.String("provider_id", oauthProviderID))
	return newUser, err
}
