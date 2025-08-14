package jwt

import (
	"NYCU-SDC/core-system-backend/internal/user"

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

const Issuer = "core-system"

type Querier interface {
	GetUserIDByTokenID(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	Create(ctx context.Context, arg CreateParams) (RefreshToken, error)
	Inactivate(ctx context.Context, id uuid.UUID) (int64, error)
	Delete(ctx context.Context) (int64, error)
	GetRefreshTokenByID(ctx context.Context, id uuid.UUID) (RefreshToken, error)
}

type Service struct {
	logger                 *zap.Logger
	secret                 string
	oauthProxySecret       string
	accessTokenExpiration  time.Duration
	refreshTokenExpiration time.Duration
	queries                Querier
	tracer                 trace.Tracer
}

func NewService(
	logger *zap.Logger,
	db DBTX,
	secret string,
	oauthProxySecret string,
	accessTokenExpiration time.Duration,
	refreshTokenExpiration time.Duration,
) *Service {
	return &Service{
		logger:                 logger,
		queries:                New(db),
		tracer:                 otel.Tracer("jwt/service"),
		secret:                 secret,
		oauthProxySecret:       oauthProxySecret,
		accessTokenExpiration:  accessTokenExpiration,
		refreshTokenExpiration: refreshTokenExpiration,
	}
}

type claims struct {
	ID        uuid.UUID
	Username  string
	Name      string
	AvatarUrl string
	Role      []string
	jwt.RegisteredClaims
}

// oauthProxyClaims defines contextual information for an OAuth transaction.
// It is encoded into the 'state' parameter as a signed JWT to preserve integrity and authenticity.
type oauthProxyClaims struct {
	// Service is the logical service requesting authentication (e.g., "core-system", "clustron").
	Service string

	// Environment represents the environment or deployment context (e.g., "pr-12", "staging").
	Environment string

	// CallbackURL is the backend endpoint to receive the OAuth authorization code.
	// It must be an internal service endpoint, not exposed to users.
	CallbackURL string

	// RedirectURL is the final URL to send the user to after authentication completes.
	// This is typically a user-facing frontend page.
	RedirectURL string

	jwt.RegisteredClaims
}

func (s Service) New(ctx context.Context, user user.User) (string, error) {
	traceCtx, span := s.tracer.Start(ctx, "New")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	jwtID := uuid.New()

	id := user.ID
	username := user.Username.String

	claims := &claims{
		ID:        jwtID,
		Username:  username,
		Name:      user.Name.String,
		AvatarUrl: user.AvatarUrl.String,
		Role:      user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Subject:   id.String(), // user id
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.accessTokenExpiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        jwtID.String(), // jwt id
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.secret))
	if err != nil {
		logger.Error("failed to sign token", zap.Error(err), zap.String("user_id", id.String()), zap.String("username", username), zap.String("role", strings.Join(user.Role, ",")))
		return "", err
	}

	logger.Debug("Generated JWT token", zap.String("id", id.String()), zap.String("username", username), zap.String("role", strings.Join(user.Role, ",")))
	return tokenString, nil
}

func (s Service) NewState(ctx context.Context, service, environment, callbackURL, redirectURL string) (string, error) {
	traceCtx, span := s.tracer.Start(ctx, "NewState")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	id := uuid.New()
	claims := &oauthProxyClaims{
		Service:     service,
		Environment: environment,
		CallbackURL: callbackURL,
		RedirectURL: redirectURL,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Subject:   id.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			NotBefore: jwt.NewNumericDate(time.Now()),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        id.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.oauthProxySecret))
	if err != nil {
		logger.Error("failed to sign state token", zap.Error(err), zap.String("service", service), zap.String("environment", environment))
		return "", err
	}

	logger.Debug("Generated OAuth proxy state token", zap.String("service", service), zap.String("environment", environment))
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

	logger.Debug("Successfully parsed JWT token", zap.String("id", tokenClaims.ID.String()), zap.String("username", tokenClaims.Username), zap.String("role", strings.Join(tokenClaims.Role, ",")))

	// Parse user ID from subject
	userID, err := uuid.Parse(tokenClaims.Subject)
	if err != nil {
		logger.Error("Failed to parse user ID from JWT subject", zap.Error(err))
		return user.User{}, err
	}

	return user.User{
		ID:        userID,
		Username:  pgtype.Text{String: tokenClaims.Username, Valid: true},
		Name:      pgtype.Text{String: tokenClaims.Name, Valid: true},
		AvatarUrl: pgtype.Text{String: tokenClaims.AvatarUrl, Valid: true},
		Role:      tokenClaims.Role,
	}, nil
}

// ParseState parses the state jwt payload to get redirect URL
func (s Service) ParseState(ctx context.Context, tokenString string) (string, error) {
	traceCtx, span := s.tracer.Start(ctx, "ParseState")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	secret := func(token *jwt.Token) (interface{}, error) {
		return []byte(s.oauthProxySecret), nil
	}

	tokenClaims := &oauthProxyClaims{}
	token, err := jwt.ParseWithClaims(tokenString, tokenClaims, secret)
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenMalformed):
			logger.Warn("Failed to parse JWT token due to malformed structure, this is not a JWT token", zap.String("token", tokenString), zap.String("error", err.Error()))
			return "", err
		case errors.Is(err, jwt.ErrSignatureInvalid):
			logger.Warn("Failed to parse JWT token due to invalid signature", zap.String("error", err.Error()))
			return "", err
		case errors.Is(err, jwt.ErrTokenExpired):
			expiredTime, getErr := token.Claims.GetExpirationTime()
			if getErr != nil {
				logger.Error("Failed to parse JWT token due to expired timestamp", zap.String("error", getErr.Error()))
				return "", err
			}
			logger.Warn("Failed to parse JWT token due to expired timestamp", zap.String("error", err.Error()), zap.Time("expired_at", expiredTime.Time))
			return "", err
		case errors.Is(err, jwt.ErrTokenNotValidYet):
			notBeforeTime, getErr := token.Claims.GetNotBefore()
			if getErr != nil {
				logger.Error("Failed to parse JWT token due to not valid yet timestamp", zap.String("error", getErr.Error()))
				return "", err
			}
			logger.Warn("Failed to parse JWT token due to not valid yet timestamp", zap.String("error", err.Error()), zap.Time("not_before", notBeforeTime.Time))
			return "", err
		default:
			logger.Error("Failed to parse JWT token", zap.Error(err))
			return "", err
		}
	}

	logger.Debug("Successfully parsed OAuth proxy state token", zap.String("service", tokenClaims.Service), zap.String("environment", tokenClaims.Environment), zap.String("callback_url", tokenClaims.CallbackURL), zap.String("redirect_url", tokenClaims.RedirectURL))

	return tokenClaims.RedirectURL, nil
}

func (s Service) GetUserIDByRefreshToken(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetUserIDByRefreshToken")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	userID, err := s.queries.GetUserIDByTokenID(ctx, id)
	if err != nil {
		logger.Error("failed to get user id by refresh token", zap.Error(err))
		return uuid.UUID{}, err
	}

	return userID, nil
}

func (s Service) GenerateRefreshToken(ctx context.Context, userID uuid.UUID) (RefreshToken, error) {
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
		UserID: userID,
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

func (s Service) GetRefreshTokenByID(ctx context.Context, id uuid.UUID) (RefreshToken, error) {
	traceCtx, span := s.tracer.Start(ctx, "GetRefreshTokenByID")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	refreshToken, err := s.queries.GetRefreshTokenByID(traceCtx, id)
	if err != nil {
		err = databaseutil.WrapDBError(err, logger, "get refresh token by id")
		span.RecordError(err)
		return RefreshToken{}, err
	}

	return refreshToken, nil
}
