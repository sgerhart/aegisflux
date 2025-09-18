Wire it all in internal/api/http.go:

- POST /plans:
  * Parse request (finding_id or inline finding). For MVP, expect inline finding.
  * Run pipeline:
      draft  := Planner.BuildPlanDraft(...)
      targets:= draft.targets (or [finding.host_id])
      controls := PolicyWriter.Materialize(draft.control_intents)
      targets   = Segmenter.MaybeExpand(targets, canaryLimit)
      strat     = Guardrails.DecideStrategy(draft.desired_mode, len(targets), finding.context.labels)
      Explain   = Explainer.Summarize(plan components)
      Plan{...} = assemble; Status="proposed"; Notes=Explain
      Store.Put(plan); NATS.PublishPlanProposed(plan)
  * Return plan JSON.
- GET /plans, GET /plans/{id}, health/ready unchanged.
Add structured JSON logs for: plan_id, targets, strategy.mode, controls.count.
