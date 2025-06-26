package jwt

import (
	"NYCU-SDC/core-system-backend/internal"
	"NYCU-SDC/core-system-backend/internal/user"
	"context"
	"fmt"
	"net/http"

	"github.com/NYCU-SDC/summer/pkg/problem"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type UserStore interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (user.User, error)
}

type Middleware struct {
	logger        *zap.Logger
	validator     *validator.Validate
	problemWriter *problem.HttpWriter
	service       *Service
	userStore     UserStore
	tracer        trace.Tracer
	debug         bool
}

// NewMiddleware creates a new user middleware
func NewMiddleware(
	logger *zap.Logger,
	validator *validator.Validate,
	problemWriter *problem.HttpWriter,
	service *Service,
	userStore UserStore,
	debug bool,
) *Middleware {
	return &Middleware{
		logger:        logger,
		validator:     validator,
		problemWriter: problemWriter,
		service:       service,
		userStore:     userStore,
		tracer:        otel.Tracer("jwt/middleware"),
		debug:         debug,
	}
}

// AuthenticateMiddleware validates JWT token and adds user to context
func (m *Middleware) AuthenticateMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		traceCtx, span := m.tracer.Start(r.Context(), "AuthenticateMiddleware")
		defer span.End()

		// Extract access token from cookie
		accessTokenCookie, err := r.Cookie("access_token")
		if err != nil {
			m.logger.Error("Missing access token cookie")
			m.problemWriter.WriteError(traceCtx, w, internal.ErrMissingAuthHeader, m.logger)
			span.RecordError(internal.ErrMissingAuthHeader)
			return
		}

		tokenString := accessTokenCookie.Value
		if tokenString == "" {
			m.logger.Error("Empty access token cookie")
			m.problemWriter.WriteError(traceCtx, w, internal.ErrInvalidAuthHeaderFormat, m.logger)
			span.RecordError(internal.ErrInvalidAuthHeaderFormat)
			return
		}

		// Parse and validate JWT token
		jwtUser, err := m.service.Parse(r.Context(), tokenString)
		if err != nil {
			m.logger.Error("Failed to parse JWT token", zap.Error(err))
			m.problemWriter.WriteError(traceCtx, w, fmt.Errorf("%w: %v", internal.ErrInvalidJWTToken, err), m.logger)
			span.RecordError(err)
			return
		}

		authenticatedUser, err := m.userStore.GetUserByID(traceCtx, jwtUser.ID)
		if err != nil {
			m.logger.Error("Failed to fetch user data from database", zap.Error(err), zap.String("user_id", jwtUser.ID.String()))
			m.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to fetch user data: %v", err), m.logger)
			span.RecordError(err)
			return
		}

		// Add authenticated user to request context
		ctxWithUser := context.WithValue(traceCtx, user.UserContextKey, &authenticatedUser)

		// Call the actual handler with authenticated context
		handler(w, r.WithContext(ctxWithUser))
	}
}
