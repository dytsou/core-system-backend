package inbox

import (
	"NYCU-SDC/core-system-backend/internal"
	"NYCU-SDC/core-system-backend/internal/user"
	"context"
	handlerutil "github.com/NYCU-SDC/summer/pkg/handler"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	pagutil "github.com/NYCU-SDC/summer/pkg/pagination"
	"github.com/NYCU-SDC/summer/pkg/problem"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"net/http"
	"time"
)

//go:generate mockery --name Store
type Store interface {
	GetAll(ctx context.Context, userId uuid.UUID) ([]GetAllRow, error)
	GetById(ctx context.Context, id uuid.UUID, userId uuid.UUID) (GetByIdRow, any, error)
	UpdateById(ctx context.Context, id uuid.UUID, userId uuid.UUID, arg UserInboxMessageFilter) (UpdateByIdRow, error)
}

type UserInboxMessageFilter struct {
	IsRead     bool `json:"isRead"`
	IsStarred  bool `json:"isStarred"`
	IsArchived bool `json:"isArchived"`
}
type InboxMessageResponse struct {
	ID        string      `json:"id"`
	PostedBy  string      `json:"postedBy"`
	Title     string      `json:"title"`
	Subtitle  string      `json:"subtitle"`
	Type      ContentType `json:"type"`
	ContentId string      `json:"contentId"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}
type Response struct {
	ID      string               `json:"id"`
	Message InboxMessageResponse `json:"message"`
	Content any                  `json:"content"`
	UserInboxMessageFilter
}

type Handler struct {
	logger        *zap.Logger
	validator     *validator.Validate
	problemWriter *problem.HttpWriter
	service       *Service
	tracer        trace.Tracer

	inboxStore Store
}

func NewHandler(
	logger *zap.Logger,
	validator *validator.Validate,
	problemWriter *problem.HttpWriter,
	inboxStore Store,
) *Handler {
	return &Handler{
		logger:        logger,
		validator:     validator,
		problemWriter: problemWriter,
		tracer:        otel.Tracer("inbox/handler"),
		inboxStore:    inboxStore,
	}
}

func (h *Handler) GetAll(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetAll")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	factory := pagutil.NewFactory[GetAllRow](200, []string{"CreatedAt"})
	request, err := factory.GetRequest(r)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	currentUser, ok := user.GetFromContext(traceCtx)
	if !ok {
		h.problemWriter.WriteError(traceCtx, w, internal.ErrNoUserInContext, logger)
		return
	}

	messages, err := h.inboxStore.GetAll(traceCtx, currentUser.ID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	response := factory.NewResponse(messages, len(messages), request.Page, request.Size)

	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}

func (h *Handler) GetById(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetById")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	pathID := r.PathValue("id")
	id, err := internal.ParseUUID(pathID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	currentUser, ok := user.GetFromContext(traceCtx)
	if !ok {
		h.problemWriter.WriteError(traceCtx, w, internal.ErrNoUserInContext, logger)
		return
	}

	message, messageContent, err := h.inboxStore.GetById(traceCtx, id, currentUser.ID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	response := Response{
		ID: message.ID.String(),
		Message: InboxMessageResponse{
			ID:        message.MessageID.String(),
			PostedBy:  message.PostedBy.String(),
			Title:     message.Title,
			Subtitle:  message.Subtitle.String,
			Type:      message.Type,
			ContentId: message.ContentID.String(),
			CreatedAt: message.CreatedAt.Time.Format(time.RFC3339),
			UpdatedAt: message.UpdatedAt.Time.Format(time.RFC3339),
		},
		Content: messageContent,
		UserInboxMessageFilter: UserInboxMessageFilter{
			IsRead:     message.IsRead.Bool,
			IsStarred:  message.IsStarred.Bool,
			IsArchived: message.IsArchived.Bool,
		},
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}

func (h *Handler) UpdateById(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "UpdateById")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	pathID := r.PathValue("id")
	id, err := internal.ParseUUID(pathID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	var req UserInboxMessageFilter
	if err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &req); err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	currentUser, ok := user.GetFromContext(traceCtx)
	if !ok {
		h.problemWriter.WriteError(traceCtx, w, internal.ErrNoUserInContext, logger)
		return
	}

	message, err := h.inboxStore.UpdateById(traceCtx, id, currentUser.ID, UserInboxMessageFilter{
		IsRead:     req.IsRead,
		IsStarred:  req.IsStarred,
		IsArchived: req.IsArchived,
	})
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	response := Response{
		ID: message.ID.String(),
		Message: InboxMessageResponse{
			ID:        message.MessageID.String(),
			PostedBy:  message.PostedBy.String(),
			Title:     message.Title,
			Subtitle:  message.Subtitle.String,
			Type:      message.Type,
			ContentId: message.ContentID.String(),
			CreatedAt: message.CreatedAt.Time.Format(time.RFC3339),
			UpdatedAt: message.UpdatedAt.Time.Format(time.RFC3339),
		},
		UserInboxMessageFilter: UserInboxMessageFilter{
			IsRead:     message.IsRead.Bool,
			IsStarred:  message.IsStarred.Bool,
			IsArchived: message.IsArchived.Bool,
		},
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}
