package user

import (
	"NYCU-SDC/core-system-backend/internal"
	"context"
	"encoding/json"
	"net/http"

	"github.com/NYCU-SDC/summer/pkg/problem"
	"github.com/go-playground/validator/v10"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// User context key to avoid import cycle with middleware package
type contextKey string

const UserContextKey contextKey = "user"

// Handler handles user-related HTTP requests
type Handler struct {
	logger        *zap.Logger
	validator     *validator.Validate
	problemWriter *problem.HttpWriter
	service       *Service
	tracer        trace.Tracer
}

// NewHandler creates a new user handler
func NewHandler(
	logger *zap.Logger,
	validator *validator.Validate,
	problemWriter *problem.HttpWriter,
	service *Service,
) *Handler {
	return &Handler{
		logger:        logger,
		validator:     validator,
		problemWriter: problemWriter,
		service:       service,
		tracer:        otel.Tracer("user/handler"),
	}
}

// GetUserFromContext extracts the authenticated user from request context
func GetUserFromContext(ctx context.Context) (*User, bool) {
	userData, ok := ctx.Value(UserContextKey).(*User)
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

// GetMe handles GET /user/me - returns authenticated user information
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "GetMe")
	defer span.End()

	// Get authenticated user from context
	user, ok := GetUserFromContext(ctx)
	if !ok {
		h.logger.Error("No user found in request context")
		h.writeNotFound(w, "user")
		return
	}

	response := UserMeResponse{
		ID:        user.ID.String(),
		Username:  user.Username,
		Name:      user.Name,
		AvatarUrl: user.AvatarUrl,
		Role:      user.Role,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// writeJSONResponse writes a JSON response
func (h *Handler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", zap.Error(err))
	}
}

// writeNotFound writes a 404 Not Found response using RFC 7807 format
func (h *Handler) writeNotFound(w http.ResponseWriter, resource string) {
	notFoundError := internal.NewNotFound(resource)
	notFoundError.ProblemDetail.Detail = "The requested " + resource + " was not found"

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusNotFound)

	if err := json.NewEncoder(w).Encode(notFoundError); err != nil {
		h.logger.Error("Failed to encode not found response", zap.Error(err))
	}
}
