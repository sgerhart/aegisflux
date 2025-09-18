Implement internal/agents/runtime.go:

- Tool dispatcher with allowlists per role:
  planner:     ["graph.query","cve.lookup","risk.context","policy.compile","ebpf.template_suggest"]
  policywriter:["policy.compile","ebpf.template_suggest","registry.sign_store"]
  segmenter:   ["graph.query","risk.context"]
  explainer:   [] (no tools; uses given context)

- JSON "function-calling" contract:
  * Each LLM step receives {input, tools:[names], constraints}, must emit {calls:[{fn, args}], draft?}.
  * Runtime executes calls in order, returns results to LLM in next turn (single- or two-shot loop with max 2 iterations).

- Respect Budget & RPM; return an explicit error if exceeded (the HTTP layer converts that to 429 or falls back to suggest plan with minimal controls).
