package agents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIRequest represents a request to OpenAI API
type OpenAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

// Message represents a message in the conversation
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse represents a response from OpenAI API
type OpenAIResponse struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a choice in the response
type Choice struct {
	Message Message `json:"message"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIError represents an error from OpenAI API
type OpenAIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// OpenAIClient implements LLMClient for OpenAI
type OpenAIClient struct {
	apiKey      string
	model       string
	maxTokens   int
	temperature float64
	provider    string
	httpClient  *http.Client
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey, model string, maxTokens int, temperature float64) *OpenAIClient {
	return &OpenAIClient{
		apiKey:      apiKey,
		model:       model,
		maxTokens:   maxTokens,
		temperature: temperature,
		provider:    "openai",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Generate generates text using OpenAI API
func (c *OpenAIClient) Generate(prompt string) (string, error) {
	request := OpenAIRequest{
		Model: c.model,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   c.maxTokens,
		Temperature: c.temperature,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiError OpenAIError
		if err := json.Unmarshal(body, &apiError); err != nil {
			return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return "", fmt.Errorf("OpenAI API error: %s", apiError.Error.Message)
	}

	var response OpenAIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return response.Choices[0].Message.Content, nil
}

// GetMaxTokens returns the maximum tokens for this client
func (c *OpenAIClient) GetMaxTokens() int {
	return c.maxTokens
}

// GetTemperature returns the temperature setting for this client
func (c *OpenAIClient) GetTemperature() float64 {
	return c.temperature
}

// GetProvider returns the provider name
func (c *OpenAIClient) GetProvider() string {
	return c.provider
}

// LocalRequest represents a request to a local OpenAI-compatible API
type LocalRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// LocalResponse represents a response from a local API
type LocalResponse struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage,omitempty"`
}

// LocalClient implements LLMClient for local OpenAI-compatible APIs (like Ollama)
type LocalClient struct {
	baseURL     string
	model       string
	maxTokens   int
	temperature float64
	provider    string
	httpClient  *http.Client
}

// NewLocalClient creates a new local client
func NewLocalClient(baseURL, model string, maxTokens int, temperature float64) *LocalClient {
	return &LocalClient{
		baseURL:     baseURL,
		model:       model,
		maxTokens:   maxTokens,
		temperature: temperature,
		provider:    "local",
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for local models
		},
	}
}

// Generate generates text using local API
func (c *LocalClient) Generate(prompt string) (string, error) {
	request := LocalRequest{
		Model: c.model,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   c.maxTokens,
		Temperature: c.temperature,
		Stream:      false,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/chat/completions", c.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("local API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response LocalResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return response.Choices[0].Message.Content, nil
}

// GetMaxTokens returns the maximum tokens for this client
func (c *LocalClient) GetMaxTokens() int {
	return c.maxTokens
}

// GetTemperature returns the temperature setting for this client
func (c *LocalClient) GetTemperature() float64 {
	return c.temperature
}

// GetProvider returns the provider name
func (c *LocalClient) GetProvider() string {
	return c.provider
}
