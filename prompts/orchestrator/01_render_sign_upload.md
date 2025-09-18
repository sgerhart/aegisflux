Implement internal/ebpf/render.go, pack.go, registry.go:
- Render(templateName, params) -> compile CO-RE object (invoke Makefile or compiler container)
- Pack -> tar.zst (program.o + metadata.json)
- Sign via registry client (POST /artifacts) → get artifact_id, signature
Return artifact reference for inclusion in Plan Controls.
