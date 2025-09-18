- internal/nats/subscriber.go:
  * Connect to NATS_URL.
  * Subscribe to "events.enriched" (queue "correlator"); also subscribe to "events.raw" and process both (enriched preferred if both present).
  * Parse JSON; on error increment correlator_events_invalid_total.
  * Call rules.Evaluate.OnEvent(ev).
  * Graceful shutdown & drain.
