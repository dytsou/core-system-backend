package jwt

import (
	user "NYCU-SDC/core-system-backend/internal/user"

	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	databaseutil "github.com/NYCU-SDC/summer/pkg/database"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/golang-jwt/jwt/v5"
)

type Store interface {
	GetUserIDByRefreshToken(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	GenerateRefreshToken(ctx context.Context, user user.User) (RefreshToken, error)
	InactivateRefreshTokens(ctx context.Context, user user.User) error
	DeleteExpiredRefreshTokens(ctx context.Context) (int64, error)
}

type Service struct {
	logger                 *zap.Logger
	secret                 string
	expiration             time.Duration
	refreshTokenExpiration time.Duration
	queries                *Queries
	tracer                 trace.Tracer
}

func NewService(
	logger *zap.Logger,
	db DBTX,
	secret string,
	expiration time.Duration,
	refreshTokenExpiration time.Duration,
) *Service {
	return &Service{
		logger:                 logger,
		queries:                New(db),
		tracer:                 otel.Tracer("jwt/service"),
		secret:                 secret,
		expiration:             expiration,
		refreshTokenExpiration: refreshTokenExpiration,
	}
}

type claims struct {
	ID       uuid.UUID
	Username string
	Role     string
	jwt.RegisteredClaims
}

func (s Service) New(ctx context.Context, user user.User) (string, error) {
	traceCtx, span := s.tracer.Start(ctx, "New")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	jwtID := uuid.New()

	id := user.ID
	username := user.Username.String
	roleStr := ""
	if len(user.Role) > 0 {
		roleStr = strings.Join(user.Role, ",")
	}

	claims := &claims{
		ID:       jwtID,
		Username: username,
		Role:     roleStr,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "core-system",
			Subject:   id.String(), // user id
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        jwtID.String(), // jwt id
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.secret))
	if err != nil {
		logger.Error("failed to sign token", zap.Error(err), zap.String("user_id", id.String()), zap.String("username", username), zap.String("role", roleStr))
		return "", err
	}

	logger.Debug("Generated JWT token", zap.String("id", id.String()), zap.String("username", username), zap.String("role", roleStr))
	return tokenString, nil
}

func (s Service) Parse(ctx context.Context, tokenString string) (user.User, error) {
	traceCtx, span := s.tracer.Start(ctx, "Parse")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	secret := func(token *jwt.Token) (interface{}, error) {
		return []byte(s.secret), nil
	}

	tokenClaims := &claims{}
	token, err := jwt.ParseWithClaims(tokenString, tokenClaims, secret)
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenMalformed):
			logger.Warn("Failed to parse JWT token due to malformed structure, this is not a JWT token", zap.String("token", tokenString), zap.String("error", err.Error()))
			return user.User{}, err
		case errors.Is(err, jwt.ErrSignatureInvalid):
			logger.Warn("Failed to parse JWT token due to invalid signature", zap.String("error", err.Error()))
			return user.User{}, err
		case errors.Is(err, jwt.ErrTokenExpired):
			expiredTime, getErr := token.Claims.GetExpirationTime()
			if getErr != nil {
				logger.Error("Failed to parse JWT token due to expired timestamp", zap.String("error", getErr.Error()))
				return user.User{}, err
			}
			logger.Warn("Failed to parse JWT token due to expired timestamp", zap.String("error", err.Error()), zap.Time("expired_at", expiredTime.Time))
			return user.User{}, err
		case errors.Is(err, jwt.ErrTokenNotValidYet):
			notBeforeTime, getErr := token.Claims.GetNotBefore()
			if getErr != nil {
				logger.Error("Failed to parse JWT token due to not valid yet timestamp", zap.String("error", getErr.Error()))
				return user.User{}, err
			}
			logger.Warn("Failed to parse JWT token due to not valid yet timestamp", zap.String("error", err.Error()), zap.Time("not_before", notBeforeTime.Time))
			return user.User{}, err
		default:
			logger.Error("Failed to parse JWT token", zap.Error(err))
			return user.User{}, err
		}
	}

	logger.Debug("Successfully parsed JWT token", zap.String("id", tokenClaims.ID.String()), zap.String("username", tokenClaims.Username), zap.String("role", tokenClaims.Role))

	roles := []string{}
	if tokenClaims.Role != "" {
		roles = strings.Split(tokenClaims.Role, ",")
	}

	// Parse user ID from subject
	userID, err := uuid.Parse(tokenClaims.Subject)
	if err != nil {
		logger.Error("Failed to parse user ID from JWT subject", zap.Error(err))
		return user.User{}, err
	}

	return user.User{
		ID:       userID,
		Username: pgtype.Text{String: tokenClaims.Username, Valid: true},
		Role:     roles,
	}, nil
}

func (s Service) GetUserIDByRefreshToken(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetUserIDByRefreshToken")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	refreshToken, err := s.queries.GetByID(ctx, id)
	if err != nil {
		logger.Error("failed to get user id by refresh token", zap.Error(err))
		return uuid.UUID{}, err
	}

	return refreshToken.UserID, nil
}

func (s Service) GenerateRefreshToken(ctx context.Context, user user.User) (RefreshToken, error) {
	traceCtx, span := s.tracer.Start(ctx, "GenerateRefreshToken")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	rowsAffected, err := s.DeleteExpiredRefreshTokens(traceCtx)
	if err != nil {
		logger.Error("failed to delete expired refresh tokens", zap.Error(err))
	}
	if rowsAffected > 0 {
		logger.Info("deleted expired refresh tokens", zap.Int64("rows_affected", rowsAffected))
	}

	expirationDate := time.Now()
	nextRefreshDate := expirationDate.Add(s.refreshTokenExpiration)

	params := CreateParams{
		UserID: user.ID,
		ExpirationDate: pgtype.Timestamptz{
			Time:  nextRefreshDate,
			Valid: true,
		},
	}
	refreshToken, err := s.queries.Create(traceCtx, params)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "generate refresh token")
		span.RecordError(err)
		return RefreshToken{}, err
	}
	return refreshToken, nil
}

func (s Service) InactivateRefreshTokens(ctx context.Context, user user.User) error {
	traceCtx, span := s.tracer.Start(ctx, "InactivateRefreshTokens")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	_, err := s.queries.Inactivate(traceCtx, user.ID)
	if err != nil {
		err = databaseutil.WrapDBErrorWithKeyValue(err, "refresh_token", "user_id", user.ID.String(), logger, "inactivate refresh token by user id")
		return err
	}

	return nil
}

func (s Service) DeleteExpiredRefreshTokens(ctx context.Context) (int64, error) {
	traceCtx, span := s.tracer.Start(ctx, "DeleteExpiredRefreshTokens")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	rowsAffected, err := s.queries.Delete(traceCtx)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "delete expired refresh tokens")
		span.RecordError(err)
		return 0, err
	}

	return rowsAffected, nil
}
