package inbox

import (
	"NYCU-SDC/core-system-backend/internal"
	"NYCU-SDC/core-system-backend/internal/form"
	"NYCU-SDC/core-system-backend/internal/user"
	"context"
	"fmt"
	"net/http"
	"time"

	databaseutil "github.com/NYCU-SDC/summer/pkg/database"
	handlerutil "github.com/NYCU-SDC/summer/pkg/handler"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	pagutil "github.com/NYCU-SDC/summer/pkg/pagination"
	"github.com/NYCU-SDC/summer/pkg/problem"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

//go:generate mockery --name Store
type Store interface {
	List(ctx context.Context, userID uuid.UUID) ([]ListRow, error)
	GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (GetByIDRow, error)
	UpdateByID(ctx context.Context, id uuid.UUID, userID uuid.UUID, arg UserInboxMessageFilter) (UpdateByIDRow, error)
}

type UserInboxMessageFilter struct {
	IsRead     bool `json:"isRead"`
	IsStarred  bool `json:"isStarred"`
	IsArchived bool `json:"isArchived"`
}

type MessageResponse struct {
	ID        string      `json:"id"`
	PostedBy  string      `json:"postedBy"`
	Title     string      `json:"title"`
	Subtitle  string      `json:"subtitle"`
	Type      ContentType `json:"type"`
	ContentID string      `json:"contentId"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

type Response struct {
	ID      string          `json:"id"`
	Message MessageResponse `json:"message"`
	UserInboxMessageFilter
}

type ResponseDetail struct {
	ID      string          `json:"id"`
	Message MessageResponse `json:"message"`
	Content any             `json:"content"`
	UserInboxMessageFilter
}

type Handler struct {
	logger        *zap.Logger
	tracer        trace.Tracer
	validator     *validator.Validate
	problemWriter *problem.HttpWriter

	store     Store
	formStore form.Store
}

func NewHandler(
	logger *zap.Logger,
	validator *validator.Validate,
	problemWriter *problem.HttpWriter,
	store Store,
	formStore form.Store,
) *Handler {
	return &Handler{
		logger:        logger,
		validator:     validator,
		problemWriter: problemWriter,
		tracer:        otel.Tracer("inbox/handler"),
		store:         store,
		formStore:     formStore,
	}
}

func mapToResponse(message ListRow) Response {
	return Response{
		ID: message.ID.String(),
		Message: MessageResponse{
			ID:        message.MessageID.String(),
			PostedBy:  message.PostedBy.String(),
			Title:     message.Title,
			Subtitle:  message.Subtitle.String,
			Type:      message.Type,
			ContentID: message.ContentID.String(),
			CreatedAt: message.CreatedAt.Time.Format(time.RFC3339),
			UpdatedAt: message.UpdatedAt.Time.Format(time.RFC3339),
		},
		UserInboxMessageFilter: UserInboxMessageFilter{
			IsRead:     message.IsRead,
			IsStarred:  message.IsStarred,
			IsArchived: message.IsArchived,
		},
	}
}

func (h *Handler) GetMessageContent(ctx context.Context, contentType ContentType, contentID uuid.UUID) (any, error) {
	traceCtx, span := h.tracer.Start(ctx, "GetMessageContent")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	switch contentType {
	case ContentTypeForm:
		currentForm, err := h.formStore.GetByID(traceCtx, contentID)
		if err != nil {
			err = databaseutil.WrapDBError(err, logger, "get form by id")
			span.RecordError(err)
			return Form{}, err
		}
		return currentForm, nil
	case ContentTypeText:
		return nil, nil
	}

	return nil, fmt.Errorf("content type %s not supported", contentType)
}

func (h *Handler) ListHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "ListHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	factory := pagutil.NewFactory[Response](200, []string{"CreatedAt"})
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

	messages, err := h.store.List(traceCtx, currentUser.ID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	mappedMessage := make([]Response, len(messages))
	for i, message := range messages {
		mappedMessage[i] = mapToResponse(message)
	}

	response := factory.NewResponse(mappedMessage, len(mappedMessage), request.Page, request.Size)

	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}

func (h *Handler) GetHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "GetHandler")
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

	message, err := h.store.GetByID(traceCtx, id, currentUser.ID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	contentID, err := internal.ParseUUID(message.ContentID.String())
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}
	messageContent, err := h.GetMessageContent(traceCtx, message.Type, contentID)
	if err != nil {
		return
	}

	response := ResponseDetail{
		ID: message.ID.String(),
		Message: MessageResponse{
			ID:        message.MessageID.String(),
			PostedBy:  message.PostedBy.String(),
			Title:     message.Title,
			Subtitle:  message.Subtitle.String,
			Type:      message.Type,
			ContentID: message.ContentID.String(),
			CreatedAt: message.CreatedAt.Time.Format(time.RFC3339),
			UpdatedAt: message.UpdatedAt.Time.Format(time.RFC3339),
		},
		Content: messageContent,
		UserInboxMessageFilter: UserInboxMessageFilter{
			IsRead:     message.IsRead,
			IsStarred:  message.IsStarred,
			IsArchived: message.IsArchived,
		},
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}

func (h *Handler) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "UpdateHandler")
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

	message, err := h.store.UpdateByID(traceCtx, id, currentUser.ID, UserInboxMessageFilter{
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
		Message: MessageResponse{
			ID:        message.MessageID.String(),
			PostedBy:  message.PostedBy.String(),
			Title:     message.Title,
			Subtitle:  message.Subtitle.String,
			Type:      message.Type,
			ContentID: message.ContentID.String(),
			CreatedAt: message.CreatedAt.Time.Format(time.RFC3339),
			UpdatedAt: message.UpdatedAt.Time.Format(time.RFC3339),
		},
		UserInboxMessageFilter: UserInboxMessageFilter{
			IsRead:     message.IsRead,
			IsStarred:  message.IsStarred,
			IsArchived: message.IsArchived,
		},
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}
