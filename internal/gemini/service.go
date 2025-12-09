package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	geminiAPIBaseURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent"
)

type Service struct {
	logger *zap.Logger
	tracer trace.Tracer
	apiKey string
	client *http.Client
}

func NewService(logger *zap.Logger, apiKey string) *Service {
	return &Service{
		logger: logger,
		tracer: otel.Tracer("gemini/service"),
		apiKey: apiKey,
		client: &http.Client{},
	}
}

// Chat sends a request to the Gemini API and returns the response
func (s *Service) Chat(ctx context.Context, req GeminiAPIRequest) (Response, error) {
	traceCtx, span := s.tracer.Start(ctx, "Chat")
	defer span.End()
	logger := logutil.WithContext(traceCtx, s.logger)

	if s.apiKey == "" {
		err := fmt.Errorf("gemini API key is not configured")
		logger.Error("gemini API key is missing", zap.Error(err))
		span.RecordError(err)
		return Response{}, err
	}

	// Marshal request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		logger.Error("failed to marshal request", zap.Error(err))
		span.RecordError(err)
		return Response{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s?key=%s", geminiAPIBaseURL, s.apiKey)
	httpReq, err := http.NewRequestWithContext(traceCtx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		logger.Error("failed to create HTTP request", zap.Error(err))
		span.RecordError(err)
		return Response{}, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	logger.Info("sending request to Gemini API", zap.String("url", url))
	resp, err := s.client.Do(httpReq)
	if err != nil {
		logger.Error("failed to send request to Gemini API", zap.Error(err))
		span.RecordError(err)
		return Response{}, fmt.Errorf("failed to send request to Gemini API: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("failed to read response body", zap.Error(err))
		span.RecordError(err)
		return Response{}, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		logger.Error("Gemini API returned error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)),
		)
		span.RecordError(fmt.Errorf("Gemini API returned status %d: %s", resp.StatusCode, string(body)))
		return Response{}, fmt.Errorf("Gemini API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var geminiResp GeminiAPIResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		logger.Error("failed to unmarshal response", zap.Error(err), zap.String("body", string(body)))
		span.RecordError(err)
		return Response{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check for blocked content
	if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
		err := fmt.Errorf("prompt was blocked: %s", geminiResp.PromptFeedback.BlockReason)
		logger.Error("prompt was blocked", zap.String("reason", geminiResp.PromptFeedback.BlockReason))
		span.RecordError(err)
		return Response{}, err
	}

	// Convert to simplified response
	response := geminiResp.ToResponse()
	logger.Info("successfully received response from Gemini API", zap.String("text_length", fmt.Sprintf("%d", len(response.Text))))

	return response, nil
}
