package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	geminiModel      = "text-embedding-004"
	geminiDimensions = 768
	geminiMaxTokens  = 2048
)

// GeminiProvider implements EmbeddingProvider using Google's text-embedding-004.
type GeminiProvider struct {
	apiKey string
	client *http.Client
}

// NewGeminiProvider creates a new Gemini embedding provider.
func NewGeminiProvider(apiKey string) (*GeminiProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("gemini provider requires GEMINI_API_KEY to be set")
	}
	return &GeminiProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

type geminiRequest struct {
	Requests []geminiEmbedRequest `json:"requests"`
}

type geminiEmbedRequest struct {
	Model   string          `json:"model"`
	Content geminiContent   `json:"content"`
	TaskType string         `json:"taskType,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Embeddings []geminiEmbeddingResult `json:"embeddings"`
	Error      *geminiError            `json:"error,omitempty"`
}

type geminiEmbeddingResult struct {
	Values []float32 `json:"values"`
}

type geminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

func (p *GeminiProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	results, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("gemini: empty response")
	}
	return results[0], nil
}

func (p *GeminiProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Truncate texts that are too long
	truncated := make([]string, len(texts))
	for i, t := range texts {
		maxChars := geminiMaxTokens * 4
		if len(t) > maxChars {
			truncated[i] = t[:maxChars]
		} else {
			truncated[i] = t
		}
	}

	// Build batch request
	modelPath := "models/" + geminiModel
	requests := make([]geminiEmbedRequest, len(truncated))
	for i, t := range truncated {
		requests[i] = geminiEmbedRequest{
			Model: modelPath,
			Content: geminiContent{
				Parts: []geminiPart{{Text: t}},
			},
			TaskType: "RETRIEVAL_DOCUMENT",
		}
	}

	body, err := json.Marshal(geminiRequest{Requests: requests})
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/%s:batchEmbedContents?key=%s", modelPath, p.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusBadRequest:
			return nil, fmt.Errorf("gemini: invalid request (400): %s", string(respBody))
		case http.StatusForbidden:
			return nil, fmt.Errorf("gemini: invalid API key (403)")
		case http.StatusTooManyRequests:
			return nil, fmt.Errorf("gemini: rate limited (429)")
		default:
			return nil, fmt.Errorf("gemini: API error %d: %s", resp.StatusCode, string(respBody))
		}
	}

	var result geminiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("gemini: unmarshal response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("gemini: %s: %s", result.Error.Status, result.Error.Message)
	}

	if len(result.Embeddings) != len(texts) {
		return nil, fmt.Errorf("gemini: expected %d embeddings, got %d", len(texts), len(result.Embeddings))
	}

	embeddings := make([][]float32, len(texts))
	for i, e := range result.Embeddings {
		embeddings[i] = e.Values
	}

	return embeddings, nil
}

func (p *GeminiProvider) Dimensions() int { return geminiDimensions }
func (p *GeminiProvider) Name() string    { return "gemini" }
