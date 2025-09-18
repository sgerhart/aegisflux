Create Go 1.21 service in backend/decision:

- cmd/decision/main.go: load env; start HTTP on DECISION_HTTP_ADDR; init NATS client; init agent runtime; wire guardrails & store.
- internal/model/plan.go: define StrategyMode, PlanStatus, Strategy, SuccessCriteria, Rollback, Control, Plan (as in Cap4 MVP).
- internal/store/memory.go: thread-safe map/list of Plans (cap 1000), NATS publisher hook.
- internal/api/http.go:
  * POST /plans (accepts {finding_id} OR inline {finding}) -> runs agentic pipeline -> returns Plan JSON
  * GET /plans, GET /plans/{id}, GET /healthz, GET /readyz
Return stub plan for now; later prompts fill in logic.
