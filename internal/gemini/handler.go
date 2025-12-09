package gemini

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"NYCU-SDC/core-system-backend/internal"

	handlerutil "github.com/NYCU-SDC/summer/pkg/handler"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/NYCU-SDC/summer/pkg/problem"
	"github.com/go-playground/validator/v10"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	maxUploadSizeBytes = 1 << 20 // 1MB cap for uploaded text files
)

type ChatOperator interface {
	Chat(ctx context.Context, req GeminiAPIRequest) (Response, error)
}

type Handler struct {
	logger        *zap.Logger
	validator     *validator.Validate
	problemWriter *problem.HttpWriter
	operator      ChatOperator
	tracer        trace.Tracer
}

func NewHandler(logger *zap.Logger, validator *validator.Validate, problemWriter *problem.HttpWriter, operator ChatOperator) *Handler {
	return &Handler{
		logger:        logger,
		validator:     validator,
		problemWriter: problemWriter,
		operator:      operator,
		tracer:        otel.Tracer("gemini/handler"),
	}
}

// ChatHandler handles POST requests to the Gemini API endpoint
func (h *Handler) ChatHandler(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "ChatHandler")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxUploadSizeBytes); err != nil {
			h.problemWriter.WriteError(traceCtx, w, err, logger)
			return
		}

		prompt := r.FormValue("prompt")
		var fileContent string

		file, _, err := r.FormFile("file")
		switch {
		case err == nil:
			defer file.Close()

			limited := io.LimitReader(file, maxUploadSizeBytes+1)
			data, readErr := io.ReadAll(limited)
			if readErr != nil {
				h.problemWriter.WriteError(traceCtx, w, readErr, logger)
				return
			}
			if int64(len(data)) > maxUploadSizeBytes {
				h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("%w: uploaded file exceeds %d bytes", internal.ErrValidationFailed, maxUploadSizeBytes), logger)
				return
			}
			fileContent = string(data)
		case err == http.ErrMissingFile:
			// no file provided; prompt may still be present
		default:
			h.problemWriter.WriteError(traceCtx, w, err, logger)
			return
		}

		if strings.TrimSpace(prompt) == "" && strings.TrimSpace(fileContent) == "" {
			h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("%w: prompt or file is required", internal.ErrValidationFailed), logger)
			return
		}

		combined := prompt
		if prompt != "" && fileContent != "" {
			combined += "\n\n"
		}
		combined += fileContent

		geminiReq := GeminiAPIRequest{
			Contents: []Content{
				{
					Parts: []Part{
						{Text: combined},
					},
				},
			},
		}

		response, err := h.operator.Chat(traceCtx, geminiReq)
		if err != nil {
			h.problemWriter.WriteError(traceCtx, w, err, logger)
			return
		}

		handlerutil.WriteJSONResponse(w, http.StatusOK, response)
		return
	}

	var request Request
	err := handlerutil.ParseAndValidateRequestBody(traceCtx, h.validator, r, &request)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	if strings.TrimSpace(request.Prompt) == "" {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("%w: prompt is required", internal.ErrValidationFailed), logger)
		return
	}

	// Convert request to Gemini API format
	geminiReq := request.ToGeminiAPIRequest()

	// Call the service
	response, err := h.operator.Chat(traceCtx, geminiReq)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}
