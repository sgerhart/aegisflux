In app/consumer.py:
- Connect to NATS (NATS_URL). Subscribe "events.raw" with queue "etl".
- For each msg:
  1) parse JSON with orjson.
  2) extract ts, host_id, event_type (log and ack-drop if missing).
  3) write_raw_event(...) to Timescale.
  4) If event_type == "connect": derive dst_host_id and call upsert_comm_edge(host_id, dst_host_id).
  5) Call enrich_event(...) and publish to "events.enriched".
- Ensure per-message timeout guards; use asyncio and semaphore( max_inflight = 100 ).
- Graceful shutdown on SIGTERM; drain NATS.
- Add a __main__ entrypoint to run the consumer.
