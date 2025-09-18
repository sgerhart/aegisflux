Add tests (can be table-driven with mock Router and Tools):

- planner produces at least one control_intent for connect→exec finding.
- policy-writer yields nft_drop for connect evidence; adds TTL.
- guardrails: enforce mode with NEVER_BLOCK label -> canary; with zero canary -> suggest.
- budget exceeded -> HTTP returns 429 OR returns suggest plan with empty controls (choose one behavior and test it).

Also implement a config switch: if LLM errors, fallback path:
  - targets = [finding.host_id]
  - controls = minimal nft_drop if connect evidence exists
  - strategy = suggest
