Implement app/writers/timescale.py:
- connect() uses psycopg with pooled connections.
- write_raw_event(ts, host_id, event_type, payload_json) inserts a row in events_raw.
- retry with tenacity on transient errors.
- Unit test with a DB stub (don’t require a real DB): validate SQL shape and parameter binding via a fake cursor.
- consumer.py should call write_raw_event for each message, BEFORE graph upsert; errors should be logged and retried (then DLQ to be added later).
