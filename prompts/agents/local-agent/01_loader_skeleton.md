Rust (or Go) agent:
- Poll AGENT_REGISTRY_URL /artifacts/for-host/{host_id} every AGENT_POLL_INTERVAL_SEC
- Fetch artifact.tar.zst; verify signature with Vault-provided pubkey (dev stub ok)
- Use libbpf/aya to load program; apply params/maps; start TTL timer to auto-unload
- Emit telemetry to NATS 'agent.telemetry': {loaded, verifier_msg, cpu_pct}
