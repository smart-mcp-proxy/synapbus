package embedding

import (
	"fmt"
	"os"
)

// ProviderInfo describes an available embedding provider.
type ProviderInfo struct {
	Name       string
	Model      string
	Dimensions int
	Configured bool
}

// AvailableProviders returns info about all supported providers and whether
// they have the required credentials configured.
func AvailableProviders(apiKey, ollamaURL string) []ProviderInfo {
	geminiKey := os.Getenv("GEMINI_API_KEY")
	return []ProviderInfo{
		{Name: "openai", Model: openAIModel, Dimensions: openAIDimensions, Configured: apiKey != ""},
		{Name: "gemini", Model: geminiModel, Dimensions: geminiDimensions, Configured: geminiKey != ""},
		{Name: "ollama", Model: ollamaDefaultModel, Dimensions: ollamaDimensions, Configured: true},
	}
}

// NewProvider creates an EmbeddingProvider based on the provider name.
func NewProvider(provider, apiKey, ollamaURL string) (EmbeddingProvider, error) {
	switch provider {
	case "openai":
		return NewOpenAIProvider(apiKey)
	case "gemini":
		geminiKey := os.Getenv("GEMINI_API_KEY")
		return NewGeminiProvider(geminiKey)
	case "ollama":
		return NewOllamaProvider(ollamaURL)
	case "":
		return nil, fmt.Errorf("no embedding provider specified")
	default:
		return nil, fmt.Errorf("unknown embedding provider: %q (supported: openai, gemini, ollama)", provider)
	}
}
