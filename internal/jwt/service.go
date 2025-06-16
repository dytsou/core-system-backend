package jwt

import (
	user "NYCU-SDC/core-system-backend/internal/user"

	"context"
	"errors"
	"fmt"
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
	GetUserIDByRefreshToken(ctx context.Context, id uuid.UUID) (string, error)
	GenerateRefreshToken(ctx context.Context, user user.User) (RefreshToken, error)
	InactivateRefreshToken(ctx context.Context, id uuid.UUID) error
	InactivateRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteExpiredRefreshTokens(ctx context.Context) error
	ExistsByID(ctx context.Context, id uuid.UUID) (bool, error)
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
	username := user.Username
	role := user.Role

	claims := &claims{
		ID:       jwtID,
		Username: username,
		Role:     role,
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
		logger.Error("failed to sign token", zap.Error(err), zap.String("user_id", id.String()), zap.String("username", username), zap.String("role", role))
		return "", err
	}

	logger.Debug("Generated JWT token", zap.String("id", id.String()), zap.String("username", username), zap.String("role", role))
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

	token, err := jwt.Parse(tokenString, secret)
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenMalformed):
			logger.Warn("Failed to parse JWT token due to malformed structure, this is not a JWT token", zap.String("token", tokenString), zap.String("error", err.Error()))
			return user.User{}, err
		case errors.Is(err, jwt.ErrSignatureInvalid):
			logger.Warn("Failed to parse JWT token due to invalid signature", zap.String("error", err.Error()))
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

	claims, ok := token.Claims.(*claims)
	if !ok {
		logger.Error("Failed to extract claims from JWT token")
		return user.User{}, fmt.Errorf("failed to extract claims from JWT token")
	}

	logger.Debug("Successfully parsed JWT token", zap.String("id", claims.ID.String()), zap.String("username", claims.Username), zap.String("role", claims.Role))

	return user.User{
		ID:       claims.ID,
		Username: claims.Username,
		Role:     claims.Role,
	}, nil
}

func (s Service) GetUserIDByRefreshToken(ctx context.Context, id uuid.UUID) (string, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetUserIDByRefreshToken")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	userID, err := s.queries.GetUserIDByRefreshToken(ctx, id)
	if err != nil {
		logger.Error("failed to get user id by refresh token", zap.Error(err))
		return "", err
	}

	return userID.String(), nil
}

func (s Service) GenerateRefreshToken(ctx context.Context, user user.User) (RefreshToken, error) {
	traceCtx, span := s.tracer.Start(ctx, "GenerateRefreshToken")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	_, err := s.queries.DeleteExpired(traceCtx)
	if err != nil {
		logger.Error("failed to delete expired refresh tokens", zap.Error(err))
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

func (s Service) InactivateRefreshToken(ctx context.Context, id uuid.UUID) error {
	traceCtx, span := s.tracer.Start(ctx, "InactivateRefreshToken")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	_, err := s.queries.Inactivate(traceCtx, id)
	if err != nil {
		err = databaseutil.WrapDBErrorWithKeyValue(err, "refresh_token", "id", id.String(), logger, "inactivate refresh token")
		return err
	}

	return nil
}

func (s Service) InactivateRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) error {
	traceCtx, span := s.tracer.Start(ctx, "InactivateRefreshTokensByUserID")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	_, err := s.queries.InactivateByUserID(traceCtx, userID)
	if err != nil {
		err = databaseutil.WrapDBErrorWithKeyValue(err, "refresh_token", "user_id", userID.String(), logger, "inactivate refresh token by user id")
		span.RecordError(err)
		return err
	}

	return nil
}

func (s Service) DeleteExpiredRefreshTokens(ctx context.Context) (int64, error) {
	traceCtx, span := s.tracer.Start(ctx, "DeleteExpiredRefreshTokens")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	rowsAffected, err := s.queries.DeleteExpired(traceCtx)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "delete expired refresh tokens")
		span.RecordError(err)
		return 0, err
	}

	return rowsAffected, nil
}

func (s Service) ExistsByID(ctx context.Context, id uuid.UUID) (bool, error) {
	traceCtx, span := s.tracer.Start(ctx, "ExistsByID")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	_, err := s.queries.GetUserIDByRefreshToken(ctx, id)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "check if refresh token exists by id")
		span.RecordError(err)
		return false, err
	}
	return true, nil
}
