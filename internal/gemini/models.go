package gemini

// Request represents the incoming request to the Gemini API endpoint.
// Prompt is optional when a file upload supplies the content instead.
type Request struct {
	Prompt string `json:"prompt"`
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
						Text: r.Prompt,
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
