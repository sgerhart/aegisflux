Add unload by artifact_id and automatic rollback:
- On telemetry thresholds (errors, verifier fail, high CPU) or orchestrator signal, unload and emit status 'rolled_back'
