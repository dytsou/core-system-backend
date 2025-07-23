package jwt

import (
	"NYCU-SDC/core-system-backend/internal"
	"context"
	"net/http"

	"go.uber.org/zap"

	"github.com/NYCU-SDC/summer/pkg/problem"
	"github.com/go-playground/validator/v10"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type Middleware struct {
	logger        *zap.Logger
	validator     *validator.Validate
	problemWriter *problem.HttpWriter
	service       *Service
	tracer        trace.Tracer
}

// NewMiddleware creates a new user middleware
func NewMiddleware(
	logger *zap.Logger,
	validator *validator.Validate,
	problemWriter *problem.HttpWriter,
	service *Service,
) *Middleware {
	return &Middleware{
		logger:        logger,
		validator:     validator,
		problemWriter: problemWriter,
		service:       service,
		tracer:        otel.Tracer("jwt/middleware"),
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
			m.problemWriter.WriteError(traceCtx, w, internal.ErrMissingAuthHeader, m.logger)
			return
		}

		tokenString := accessTokenCookie.Value
		if tokenString == "" {
			m.problemWriter.WriteError(traceCtx, w, internal.ErrInvalidAuthHeaderFormat, m.logger)
			return
		}

		// Parse and validate JWT token
		authenticatedUser, err := m.service.Parse(r.Context(), tokenString)
		if err != nil {
			m.problemWriter.WriteError(traceCtx, w, internal.ErrInvalidAuthUser, m.logger)
			return
		}

		// Add authenticated user to request context
		ctxWithUser := context.WithValue(traceCtx, internal.UserContextKey, &authenticatedUser)

		// Call the actual handler with authenticated context
		handler(w, r.WithContext(ctxWithUser))
	}
}
