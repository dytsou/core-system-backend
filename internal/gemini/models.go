package gemini

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Request represents the incoming request to the Gemini API endpoint.
// Prompt is optional when a file upload supplies the content instead.
type Request struct {
	Prompt json.RawMessage `json:"prompt"`
}

// GeminiAPIRequest represents the request format for Gemini API
type GeminiAPIRequest struct {
	Contents []Content `json:"contents"`
}

// Content represents a content object in Gemini API request
type Content struct {
	Parts []Part `json:"parts"`
	Role  string `json:"role,omitempty"`
}

// Part represents a part object in Gemini API request
type Part struct {
	Text string `json:"text,omitempty"`
}

// ToGeminiAPIRequest converts a Request to GeminiAPIRequest format
func (r *Request) ToGeminiAPIRequest() GeminiAPIRequest {
	return GeminiAPIRequest{
		Contents: []Content{
			{
				Parts: []Part{
					{
						Text: string(r.Prompt),
					},
				},
			},
		},
	}
}

// Response represents the response from the Gemini API endpoint
type Response struct {
	Text string `json:"text"`
}

// GeminiAPIResponse represents the full response from Gemini API
type GeminiAPIResponse struct {
	Candidates     []Candidate     `json:"candidates"`
	PromptFeedback *PromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *UsageMetadata  `json:"usageMetadata,omitempty"`
	ModelVersion   string          `json:"modelVersion,omitempty"`
	ResponseID     string          `json:"responseId,omitempty"`
}

// Candidate represents a candidate response from Gemini API
type Candidate struct {
	Content      Content `json:"content"`
	FinishReason string  `json:"finishReason"`
	Index        int     `json:"index"`
}

// PromptFeedback represents feedback about the prompt
type PromptFeedback struct {
	BlockReason   string         `json:"blockReason,omitempty"`
	SafetyRatings []SafetyRating `json:"safetyRatings,omitempty"`
}

// SafetyRating represents a safety rating
type SafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

// UsageMetadata represents token usage information
type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// ToResponse converts a GeminiAPIResponse to a simplified Response
func (g *GeminiAPIResponse) ToResponse() Response {
	if len(g.Candidates) > 0 && len(g.Candidates[0].Content.Parts) > 0 {
		return Response{
			Text: g.Candidates[0].Content.Parts[0].Text,
		}
	}
	return Response{
		Text: "",
	}
}

// AnalysisMode represents the classification mode from triage stage
type AnalysisMode string

const (
	ModeClientConfig           AnalysisMode = "MODE_CLIENT_CONFIG"
	ModeDatabaseLogic          AnalysisMode = "MODE_DATABASE_LOGIC"
	ModePerformanceConcurrency AnalysisMode = "MODE_PERFORMANCE_CONCURRENCY"
)

// TriageRequest represents the request for Stage 1 triage analysis
type TriageRequest struct {
	SystemInstruction string `json:"system_instruction"`
	FileContent       string `json:"file_content"`
}

// TriageResponse represents the JSON response from Stage 1 triage
type TriageResponse struct {
	AnalysisMode     AnalysisMode `json:"analysis_mode"`
	DetectedKeywords []string     `json:"detected_keywords"`
	PrimaryErrorLog  string       `json:"primary_error_log"`
}

// ExpertRequest represents the request for Stage 2 expert analysis
type ExpertRequest struct {
	SystemInstruction string `json:"system_instruction"`
	FileContent       string `json:"file_content"`
}

// ExpertResponse represents the response from Stage 2 expert analysis
// This is a free-form text response, not structured JSON
type ExpertResponse struct {
	Text string `json:"text"`
}

// AnalyzeLogRequest represents the request for the two-stage log analysis
type AnalyzeLogRequest struct {
	TriagePrompt  string            `json:"triage_prompt" validate:"required"`  // Stage 1 prompt
	ExpertPrompts map[string]string `json:"expert_prompts" validate:"required"` // Stage 2 prompts: key is analysis_mode, value is prompt
	FileContent   string            `json:"file_content" validate:"required"`   // Log file content
}

// ParseTriageResponse attempts to parse a JSON response from the triage stage
// It handles both pure JSON and JSON wrapped in markdown code blocks
func ParseTriageResponse(text string) (*TriageResponse, error) {
	// Remove markdown code blocks if present
	cleaned := strings.TrimSpace(text)
	if strings.HasPrefix(cleaned, "```json") {
		cleaned = strings.TrimPrefix(cleaned, "```json")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
	} else if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
	}

	var triageResp TriageResponse
	if err := json.Unmarshal([]byte(cleaned), &triageResp); err != nil {
		return nil, err
	}

	return &triageResp, nil
}

// GetExpertPrompt returns the appropriate expert prompt from the provided map based on the analysis mode
func GetExpertPrompt(expertPrompts map[string]string, mode AnalysisMode) (string, error) {
	modeStr := string(mode)
	prompt, exists := expertPrompts[modeStr]
	if !exists {
		return "", fmt.Errorf("expert prompt not found for mode: %s", modeStr)
	}
	return prompt, nil
}
