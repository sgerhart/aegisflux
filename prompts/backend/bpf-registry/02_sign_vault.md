Implement internal/sign/vault.go:
- Read signer key (fake in dev) from VAULT_ADDR + BPF_REGISTRY_SIGNER_PATH
- sign(data) → base64 signature
- Store signature in metadata.json
Unit tests: return deterministic signature for fixture input (stub hash ok).
