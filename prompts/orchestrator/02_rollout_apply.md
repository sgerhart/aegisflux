Implement internal/rollout/ebpf.go:
- ApplyCanary(plan_id, targets[], artifact_id, ttl) -> push canary assignment (e.g., POST to registry listing or publish NATS subject 'actions.apply.ebpf')
- Wait/observe telemetry (subscribe NATS 'agent.telemetry') for a window; if violations -> rollback; else mark success.
- Expose /apply/ebpf (internal API) to kick canary/enforce; integrate with existing /apply path.
