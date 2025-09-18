Implement rule schema & loader:
- internal/rules/model.go structs: Selector, EventPattern, Condition, Outcome, Dedupe, Rule (as described).
- internal/rules/loader.go:
  * Load YAML/JSON from CORR_RULES_DIR sorted by filename.
  * Skip disabled; validate required fields; return snapshot []Rule.
  * If CORR_HOT_RELOAD=true, watch directory with debounce and refresh snapshot.
- Unit tests: load valid; skip disabled; filename override wins on same metadata.id.
