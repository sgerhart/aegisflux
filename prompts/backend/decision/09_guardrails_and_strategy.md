Complete internal/guardrails/guardrails.go:

- DecideStrategy(desired string, numTargets int, hostLabels []string) -> Strategy
  Rules:
    - Maintenance window active -> downgrade enforce→canary→suggest→observe
    - Any target has NEVER_BLOCK_LABELS -> cap at canary (or suggest if canary_size==0)
    - canary_size = min(DECISION_MAX_CANARY_HOSTS, numTargets); if 0 in enforce -> suggest
    - Set TTL from DECISION_DEFAULT_TTL_SECONDS
Unit tests for downgrades and canary sizing.
