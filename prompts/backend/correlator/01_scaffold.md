Create Go 1.21 service with:
- cmd/correlator/main.go: load env, start HTTP at CORR_HTTP_ADDR, start NATS subscriber, wire rule loader snapshot & overrides.
- internal/model/model.go: Event (ts, host_id, event_type, binary_path, args map[string]any, context map[string]any); Finding (id,severity,confidence,status,host_id,cve,evidence[],ts,rule_id,ttl_seconds).
- internal/store/memory.go: thread-safe ring buffer for Findings (cap CORR_MAX_FINDINGS) + LRU for dedupe keys (cap CORR_DEDUPE_CAP).
- internal/api/http.go: GET /findings, POST /findings/reset, GET /healthz, GET /readyz.
README: run instructions.
