# bpf-registry
Artifact registry for signed eBPF mitigation bundles. 
- Stores CO-RE object files, metadata, and signatures. 
- Provides REST/gRPC for host agents to fetch verified artifacts.
- Integrates with Vault for signing/verification keys.
