CREATE TABLE IF NOT EXISTS events_raw (
  ts timestamptz NOT NULL,
  host_id text NOT NULL,
  event_type text NOT NULL,
  payload jsonb NOT NULL,
  PRIMARY KEY (ts, host_id, event_type)
);
SELECT create_hypertable('events_raw','ts', if_not_exists => TRUE);

-- Helpful indexes
CREATE INDEX IF NOT EXISTS events_raw_ts_idx ON events_raw (ts DESC);
CREATE INDEX IF NOT EXISTS events_raw_host_idx ON events_raw (host_id);
CREATE INDEX IF NOT EXISTS events_raw_type_idx ON events_raw (event_type);
