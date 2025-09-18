Implement internal/agents/policy_writer.go:

- From control_intents[], call policy.compile and optionally ebpf.template_suggest when intent.type == "ebpf_mitigate".
- Produce []model.Control with Mode="simulate" and attach artifact previews (if available).
- Limit scope by intent (pid/cgroup/host); always include TTL from DECISION_DEFAULT_TTL_SECONDS.
