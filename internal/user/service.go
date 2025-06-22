package user

import (
	"context"
	"strings"

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

func NewService(logger *zap.Logger, queries *Queries, tracer trace.Tracer) *Service {
	return &Service{
		logger:  logger,
		queries: queries,
		tracer:  otel.Tracer("user/service"),
	}
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

func resolveAvatarUrl(name, avatarUrl string) string {
	if avatarUrl == "" {
		return "https://ui-avatars.com/api/?name=" + name
	}
	return avatarUrl
}

func (s *Service) FindOrCreate(ctx context.Context, name, username, avatarUrl string, role []string, oauthProvider, oauthProviderID string) (User, error) {
	traceCtx, span := s.tracer.Start(ctx, "user.FindOrCreate")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	// First, try to find existing user by auth
	existingUser, err := s.queries.GetUserByAuth(ctx, GetUserByAuthParams{
		Provider:   oauthProvider,
		ProviderID: oauthProviderID,
	})
	if err == nil {
		// User exists, return it
		logger.Debug("Found existing user", zap.String("provider", oauthProvider), zap.String("provider_id", oauthProviderID))
		return existingUser, nil
	}

	// Check if it's just "user not found" (expected for new users)
	if strings.Contains(err.Error(), "no rows in result set") {
		logger.Debug("User not found, creating new user", zap.String("provider", oauthProvider), zap.String("provider_id", oauthProviderID))
		// User doesn't exist, proceed to create new user (this is expected)
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
		// Note: User was created but auth wasn't - this might need transaction handling
		return User{}, err
	}

	logger.Debug("Created auth entry", zap.String("user_id", newUser.ID.String()), zap.String("provider", oauthProvider), zap.String("provider_id", oauthProviderID))
	return newUser, nil
}
