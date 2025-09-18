- internal/rules/matcher.go:
  * EffectiveRulesFor(hostID string, labels []string, rules []Rule) []Rule
  * host_globs via path.Match; labels require ALL listed to be present.
  * apply exclude_host_ids.
- Unit tests: host match, glob match, labels-all, exclude works.
