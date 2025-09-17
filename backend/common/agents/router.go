package agents

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

// LLMClient defines the interface for LLM clients
type LLMClient interface {
	// Generate generates text based on the given prompt
	Generate(prompt string) (string, error)
	// GetMaxTokens returns the maximum tokens for this client
	GetMaxTokens() int
	// GetTemperature returns the temperature setting for this client
	GetTemperature() float64
	// GetProvider returns the provider name
	GetProvider() string
}

// ProviderConfig represents configuration for an LLM provider
type ProviderConfig struct {
	Provider    string  `json:"provider"`
	APIKey      string  `json:"api_key,omitempty"`
	BaseURL     string  `json:"base_url,omitempty"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
	Model       string  `json:"model,omitempty"`
}

// RoleConfig represents configuration for a specific role
type RoleConfig struct {
	Role        string           `json:"role"`
	Provider    string           `json:"provider"`
	MaxTokens   int              `json:"max_tokens"`
	Temperature float64          `json:"temperature"`
	Providers   []ProviderConfig `json:"providers"`
}

// RouterConfig represents the overall router configuration
type RouterConfig struct {
	Roles []RoleConfig `json:"roles"`
}

// Router manages LLM client routing based on role and configuration
type Router struct {
	config     RouterConfig
	logger     *slog.Logger
	clients    map[string]LLMClient
	providers  map[string]ProviderConfig
}

// NewRouter creates a new LLM router
func NewRouter(logger *slog.Logger) (*Router, error) {
	router := &Router{
		logger:    logger,
		clients:   make(map[string]LLMClient),
		providers: make(map[string]ProviderConfig),
	}

	// Load configuration from environment
	if err := router.loadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load router config: %w", err)
	}

	// Initialize providers
	if err := router.initializeProviders(); err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}

	return router, nil
}

// loadConfig loads the router configuration from LLM_ROUTE_CONFIG
func (r *Router) loadConfig() error {
	configPath := os.Getenv("LLM_ROUTE_CONFIG")
	if configPath == "" {
		// Use default configuration if no config file specified
		r.config = r.getDefaultConfig()
		r.logger.Info("Using default router configuration")
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	if err := json.Unmarshal(data, &r.config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	r.logger.Info("Loaded router configuration", "config_file", configPath, "roles", len(r.config.Roles))
	return nil
}

// getDefaultConfig returns a default configuration
func (r *Router) getDefaultConfig() RouterConfig {
	return RouterConfig{
		Roles: []RoleConfig{
			{
				Role:        "planner",
				Provider:    "openai",
				MaxTokens:   4000,
				Temperature: 0.7,
				Providers: []ProviderConfig{
					{
						Provider:    "openai",
						MaxTokens:   4000,
						Temperature: 0.7,
						Model:       "gpt-4",
					},
					{
						Provider:    "local",
						MaxTokens:   4000,
						Temperature: 0.7,
						BaseURL:     "http://localhost:11434",
					},
				},
			},
			{
				Role:        "explainer",
				Provider:    "openai",
				MaxTokens:   2000,
				Temperature: 0.5,
				Providers: []ProviderConfig{
					{
						Provider:    "openai",
						MaxTokens:   2000,
						Temperature: 0.5,
						Model:       "gpt-3.5-turbo",
					},
					{
						Provider:    "local",
						MaxTokens:   2000,
						Temperature: 0.5,
						BaseURL:     "http://localhost:11434",
					},
				},
			},
			{
				Role:        "policy-writer",
				Provider:    "openai",
				MaxTokens:   3000,
				Temperature: 0.3,
				Providers: []ProviderConfig{
					{
						Provider:    "openai",
						MaxTokens:   3000,
						Temperature: 0.3,
						Model:       "gpt-4",
					},
					{
						Provider:    "local",
						MaxTokens:   3000,
						Temperature: 0.3,
						BaseURL:     "http://localhost:11434",
					},
				},
			},
			{
				Role:        "segmenter",
				Provider:    "openai",
				MaxTokens:   1500,
				Temperature: 0.4,
				Providers: []ProviderConfig{
					{
						Provider:    "openai",
						MaxTokens:   1500,
						Temperature: 0.4,
						Model:       "gpt-3.5-turbo",
					},
					{
						Provider:    "local",
						MaxTokens:   1500,
						Temperature: 0.4,
						BaseURL:     "http://localhost:11434",
					},
				},
			},
		},
	}
}

// initializeProviders initializes the provider configurations
func (r *Router) initializeProviders() error {
	for _, role := range r.config.Roles {
		for _, provider := range role.Providers {
			// Store provider config
			r.providers[provider.Provider] = provider
			
			// Create client for this role-provider combination
			client, err := r.createClient(provider, role)
			if err != nil {
				r.logger.Warn("Failed to create client, skipping", 
					"role", role.Role, 
					"provider", provider.Provider, 
					"error", err)
				continue
			}
			
			clientKey := fmt.Sprintf("%s:%s", role.Role, provider.Provider)
			r.clients[clientKey] = client
			r.logger.Debug("Created client", "role", role.Role, "provider", provider.Provider)
		}
	}
	
	if len(r.clients) == 0 {
		return fmt.Errorf("no clients could be initialized - check your configuration and API keys")
	}
	
	r.logger.Info("Initialized providers", "provider_count", len(r.providers), "client_count", len(r.clients))
	return nil
}

// createClient creates an LLM client based on the provider configuration
func (r *Router) createClient(provider ProviderConfig, role RoleConfig) (LLMClient, error) {
	switch provider.Provider {
	case "openai":
		return r.createOpenAIClient(provider, role)
	case "local":
		return r.createLocalClient(provider, role)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider.Provider)
	}
}

// createOpenAIClient creates an OpenAI client
func (r *Router) createOpenAIClient(provider ProviderConfig, role RoleConfig) (LLMClient, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}
	
	client := &OpenAIClient{
		apiKey:      apiKey,
		model:       provider.Model,
		maxTokens:   role.MaxTokens,
		temperature: role.Temperature,
		provider:    "openai",
	}
	
	r.logger.Debug("Created OpenAI client", "role", role.Role, "model", provider.Model, "max_tokens", role.MaxTokens)
	return client, nil
}

// createLocalClient creates a local OpenAI-compatible client
func (r *Router) createLocalClient(provider ProviderConfig, role RoleConfig) (LLMClient, error) {
	baseURL := provider.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434" // Default Ollama URL
	}
	
	client := &LocalClient{
		baseURL:     baseURL,
		model:       provider.Model,
		maxTokens:   role.MaxTokens,
		temperature: role.Temperature,
		provider:    "local",
	}
	
	r.logger.Debug("Created local client", "role", role.Role, "base_url", baseURL, "max_tokens", role.MaxTokens)
	return client, nil
}

// ClientFor returns an LLM client for the specified role
func (r *Router) ClientFor(role string) (LLMClient, error) {
	// Find role configuration
	var roleConfig *RoleConfig
	for _, r := range r.config.Roles {
		if r.Role == role {
			roleConfig = &r
			break
		}
	}
	
	if roleConfig == nil {
		return nil, fmt.Errorf("role not found: %s", role)
	}
	
	// Try to get client for the primary provider first
	clientKey := fmt.Sprintf("%s:%s", role, roleConfig.Provider)
	if client, exists := r.clients[clientKey]; exists {
		r.logger.Debug("Using primary provider", "role", role, "provider", roleConfig.Provider)
		return client, nil
	}
	
	// Fallback to any available provider for this role
	for _, provider := range roleConfig.Providers {
		clientKey := fmt.Sprintf("%s:%s", role, provider.Provider)
		if client, exists := r.clients[clientKey]; exists {
			r.logger.Debug("Using fallback provider", "role", role, "provider", provider.Provider)
			return client, nil
		}
	}
	
	return nil, fmt.Errorf("no client available for role: %s", role)
}

// GetAvailableRoles returns a list of available roles
func (r *Router) GetAvailableRoles() []string {
	var roles []string
	for _, role := range r.config.Roles {
		roles = append(roles, role.Role)
	}
	return roles
}

// GetRoleConfig returns the configuration for a specific role
func (r *Router) GetRoleConfig(role string) (*RoleConfig, error) {
	for _, r := range r.config.Roles {
		if r.Role == role {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("role not found: %s", role)
}
