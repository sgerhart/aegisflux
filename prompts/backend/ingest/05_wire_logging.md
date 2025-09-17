Wire Validator + Publisher in server:
- On each Event: Validate → Publish → count metrics; log at info with fields {host_id, event_type}.
- On validation failure: increment events_invalid_total and log warning.
- On publish failure: increment nats_publish_errors_total and return gRPC Unavailable.
- Graceful shutdown on SIGINT/SIGTERM; close NATS.
- Sane timeouts (per-event publish context timeout 2s).
