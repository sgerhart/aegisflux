Create backend/common/agents/router.go and budget.go:

- router.go: load LLM_ROUTE_CONFIG; support providers "openai" and generic "local" (OpenAI-compatible endpoint).
  Provide: func ClientFor(role string) (LLMClient, error) with MaxTokens & Temperature.

- budget.go: simple per-hour cost budget & RPM limiter:
  * Track estimated cost by tokens using a flat $/1K tokens map; enforce AGENT_MAX_COST_USD_PER_HOUR and AGENT_RATE_RPM.
  * On exceed, return BudgetExceeded error.

Expose both via decision/internal/agents/runtime.go (Runtime struct with Router+Budget).
