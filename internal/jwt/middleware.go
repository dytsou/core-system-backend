package jwt

import (
	"NYCU-SDC/core-system-backend/internal"
	"NYCU-SDC/core-system-backend/internal/user"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/NYCU-SDC/summer/pkg/problem"
	"github.com/go-playground/validator/v10"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// User context key to avoid import cycle with middleware package
type contextKey string

const UserContextKey contextKey = "user"

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

// GetUserFromContext extracts the authenticated user from request context
func GetUserFromContext(ctx context.Context) (*user.User, bool) {
	userData, ok := ctx.Value(UserContextKey).(*user.User)
	return userData, ok
}

// UserMeResponse represents the response format for /user/me endpoint
type UserMeResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	AvatarUrl string `json:"avatarUrl"`
	Role      string `json:"role"`
}

func (m *Middleware) HandlerFunc(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	}
}

// GetMe handles GET /user/me - returns authenticated user information
func (m *Middleware) GetMe(w http.ResponseWriter, r *http.Request) {
	ctx, span := m.tracer.Start(r.Context(), "GetMe")
	defer span.End()

	// Get authenticated user from context
	currentUser, ok := GetUserFromContext(ctx)
	if !ok {
		m.logger.Error("No user found in request context")
		m.writeNotFound(w, "user")
		return
	}

	// Convert roles array to comma-separated string
	roleStr := ""
	if len(currentUser.Role) > 0 {
		roleStr = strings.Join(currentUser.Role, ",")
	}

	response := UserMeResponse{
		ID:        currentUser.ID.String(),
		Username:  currentUser.Username.String,
		Name:      currentUser.Name.String,
		AvatarUrl: currentUser.AvatarUrl.String,
		Role:      roleStr,
	}

	m.writeJSONResponse(w, http.StatusOK, response)
}

// writeJSONResponse writes a JSON response
func (m *Middleware) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		m.logger.Error("Failed to encode JSON response", zap.Error(err))
	}
}

// writeNotFound writes a 404 Not Found response using RFC 7807 format
func (m *Middleware) writeNotFound(w http.ResponseWriter, resource string) {
	notFoundError := internal.NewNotFound(resource)
	notFoundError.Detail = "The requested " + resource + " was not found"

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusNotFound)

	if err := json.NewEncoder(w).Encode(notFoundError); err != nil {
		m.logger.Error("Failed to encode not found response", zap.Error(err))
	}
}
