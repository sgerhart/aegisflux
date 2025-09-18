Decision service:
- On boot, GET config snapshot from config-api; override env defaults (mode, canary, TTL, never_block_labels).
- Subscribe NATS "config.changed"; live-apply to guardrails without restart.
- If config-api unavailable, continue with env defaults and log a warning.
