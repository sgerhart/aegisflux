Create:
- rules.d/00-defaults.yaml: global bash-exec-after-connect (window_seconds=5, severity=high)
- rules.d/20-db-only.yaml: selectors.labels ["role:db"], window_seconds=3, severity=critical
- rules.d/90-host-overrides.example.yaml: host_globs ["web-*"], outcome.severity=medium (example)
Ensure loader parses them.
