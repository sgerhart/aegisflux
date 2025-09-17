Add an HTTP server on INGEST_HTTP_ADDR that exposes:
- GET /healthz → 200 {"ok":true} if both gRPC and NATS client are up
- GET /readyz → 200 once NATS is connected and schema is compiled
- GET /metrics → prometheus metrics (use promhttp)
Expose simple counters: events_total, events_invalid_total, nats_publish_errors_total.
