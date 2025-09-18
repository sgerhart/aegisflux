Implement internal/agents/planner.go:

- BuildPlanDraft(ctx, finding map[string]any) -> (draft struct):
  1) Get LLM client for "planner".
  2) Seed with system prompt (llm/system-prompts/planner.md) and a compact finding summary.
  3) First round: LLM proposes tool calls; execute allowed tools.
  4) Second round: LLM receives tool results and returns a draft:
     { targets[], control_intents[], desired_mode, success_criteria, notes }

- Validation:
  * Ensure targets non-empty; otherwise fallback to [finding.host_id].
  * Prune control_intents to reasonable max (e.g., 3).
  * desired_mode ∈ {observe,suggest,canary,enforce}; else "suggest".
Return the draft.
