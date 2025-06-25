package user

import (
	"context"
	"net/url"

	databaseutil "github.com/NYCU-SDC/summer/pkg/database"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Querier interface {
	GetUserIDByID(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (User, error)
	GetUserIDByAuth(ctx context.Context, arg GetUserIDByAuthParams) (uuid.UUID, error)
	UserExistsByAuth(ctx context.Context, arg UserExistsByAuthParams) (bool, error)
	CreateUser(ctx context.Context, arg CreateUserParams) (User, error)
	CreateAuth(ctx context.Context, arg CreateAuthParams) (Auth, error)
}

type Service struct {
	logger  *zap.Logger
	queries Querier
	tracer  trace.Tracer
}

func NewService(logger *zap.Logger, db DBTX) *Service {
	return &Service{
		logger:  logger,
		queries: New(db),
		tracer:  otel.Tracer("user/service"),
	}
}

func (s *Service) GetUserIDByID(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetUserIDByID")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	userID, err := s.queries.GetUserIDByID(traceCtx, id)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "get user by id")
		span.RecordError(err)
		return uuid.UUID{}, err
	}
	return userID, nil
}

func (s *Service) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetUserByID")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	user, err := s.queries.GetUserByID(traceCtx, id)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "get user by id")
		span.RecordError(err)
		return User{}, err
	}
	return user, nil
}

func resolveAvatarUrl(name, avatarUrl string) string {
	if avatarUrl == "" {
		return "https://ui-avatars.com/api/?name=" + url.QueryEscape(name)
	}
	return avatarUrl
}

func (s *Service) FindOrCreate(ctx context.Context, name, username, avatarUrl string, role []string, oauthProvider, oauthProviderID string) (uuid.UUID, error) {
	traceCtx, span := s.tracer.Start(ctx, "FindOrCreate")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	userExists, err := s.queries.UserExistsByAuth(traceCtx, UserExistsByAuthParams{
		Provider:   oauthProvider,
		ProviderID: oauthProviderID,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "check user existence by auth")
		span.RecordError(err)
		return uuid.UUID{}, err
	}

	if userExists {
		existingUserID, err := s.queries.GetUserIDByAuth(traceCtx, GetUserIDByAuthParams{
			Provider:   oauthProvider,
			ProviderID: oauthProviderID,
		})
		if err != nil {
			err = databaseutil.WrapDBError(err, logger, "get user by auth")
			span.RecordError(err)
			return uuid.UUID{}, err
		}
		logger.Debug("Found existing user", zap.String("provider", oauthProvider), zap.String("provider_id", oauthProviderID))
		return existingUserID, nil
	}

	// User doesn't exist, create new user
	logger.Debug("User not found, creating new user", zap.String("provider", oauthProvider), zap.String("provider_id", oauthProviderID))
	avatarUrl = resolveAvatarUrl(name, avatarUrl)
	if len(role) == 0 {
		role = []string{"user"}
	}

	newUser, err := s.queries.CreateUser(traceCtx, CreateUserParams{
		Name:      pgtype.Text{String: name, Valid: name != ""},
		Username:  pgtype.Text{String: username, Valid: username != ""},
		AvatarUrl: pgtype.Text{String: avatarUrl, Valid: avatarUrl != ""},
		Role:      role,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "create user")
		span.RecordError(err)
		return uuid.UUID{}, err
	}

	logger.Debug("Created new user", zap.String("user_id", newUser.ID.String()), zap.String("username", newUser.Username.String))

	// Create auth entry
	_, err = s.queries.CreateAuth(traceCtx, CreateAuthParams{
		UserID:     newUser.ID,
		Provider:   oauthProvider,
		ProviderID: oauthProviderID,
	})
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "create auth")
		span.RecordError(err)
		return uuid.UUID{}, err
	}

	logger.Debug("Created auth entry", zap.String("user_id", newUser.ID.String()), zap.String("provider", oauthProvider), zap.String("provider_id", oauthProviderID))
	return newUser.ID, err
}
