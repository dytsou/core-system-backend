package user

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	handlerutil "github.com/NYCU-SDC/summer/pkg/handler"
	"github.com/NYCU-SDC/summer/pkg/problem"
	"github.com/go-playground/validator/v10"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// User context key to avoid import cycle with middleware package
type contextKey string

const UserContextKey contextKey = "user"

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

type Handler struct {
	logger        *zap.Logger
	validator     *validator.Validate
	problemWriter *problem.HttpWriter
	service       *Service
	tracer        trace.Tracer
}

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

// GetMe handles GET /user/me - returns authenticated user information
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetMe")
	defer span.End()

	// Extract trace_id from context
	traceID := trace.SpanContextFromContext(traceCtx).TraceID().String()

	// Get authenticated user from context
	currentUser, ok := GetUserFromContext(traceCtx)
	if !ok {
		h.logger.Error("No user found in request context", zap.String("trace_id", traceID))
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("no user found in request context"), h.logger)
		span.RecordError(fmt.Errorf("no user found in request context"))
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

	h.logger.Debug("GetMe: Response",
		zap.String("trace_id", traceID),
		zap.String("response_id", response.ID),
		zap.String("response_username", response.Username),
		zap.String("response_name", response.Name),
		zap.String("response_avatar_url", response.AvatarUrl),
		zap.String("response_role", response.Role),
	)

	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}
