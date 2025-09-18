On Plan with control.type == 'ebpf_*':
- Call renderer → registry upload → update Plan.Control.ArtifactID
- During canary/enforce, call rollout.ApplyCanary/ApplyEnforce; handle rollback.
