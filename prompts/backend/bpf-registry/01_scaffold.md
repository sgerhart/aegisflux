Scaffold Go 1.21 service (backend/bpf-registry):
- /healthz
- POST /artifacts (multipart or JSON+base64) → store to /data/artifacts/<id>.tar.zst; write metadata.json; sign with Vault key (stub OK)
- GET /artifacts/{id} → metadata
- GET /artifacts/{id}/binary → tar.zst
- GET /artifacts/for-host/{host_id} → return list (empty MVP)
- Dockerfile + README
