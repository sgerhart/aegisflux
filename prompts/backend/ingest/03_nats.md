Implement a NATS publisher:
- internal/nats/publish.go connects to NATS_URL at startup and exposes PublishEvent(ctx, e *ingest.Event) error
- Subject: "events.raw"
- Message payload: JSON-encoded Event (as-is)
- Include headers: "x-host-id": e.host_id (if present)
- Implement lazy reconnect and a readiness check; fail fast if cannot connect on startup.
- Unit test: mock NATS (or use a test container guard) for a simple publish call; if test container is complex, stub publisher and test wiring in server with a fake Publisher.
- Update grpc.go to use the real publisher.
