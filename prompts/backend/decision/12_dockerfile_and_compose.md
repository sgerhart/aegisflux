Create multi-stage Dockerfile:
- Build with golang:1.21-alpine; copy llm/system-prompts/ & llm/models.yaml into image.
- Run on distroless; EXPOSE 8083.

Update compose:
  decision:
    build: ../../backend/decision
    env_file: ["../../.env"]
    depends_on: [nats, neo4j]
    ports: ["8083:8083"]

Smoke test:
curl -s -X POST :8083/plans -H 'content-type: application/json' -d '{
  "finding": {
    "id":"F-DEMO-1",
    "severity":"high",
    "host_id":"host-001",
    "evidence":[ "connect:10.0.0.10:5432@2025-09-16T12:00:00Z", "exec:/bin/sh@2025-09-16T12:00:03Z" ],
    "status":"open",
    "ts":"2025-09-16T12:00:03Z",
    "context": { "labels": ["env:prod","role:web"] }
  }
}'
Expect: plan with targets, controls (nft_drop preview), strategy (suggest/canary), and notes.
