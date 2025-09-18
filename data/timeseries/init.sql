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

CREATE TABLE IF NOT EXISTS app_config (
  key         text PRIMARY KEY,
  value       jsonb NOT NULL,
  scope       text NOT NULL DEFAULT 'global',  -- global|env:prod|host:web-01|role:web
  updated_by  text NOT NULL DEFAULT 'system',
  updated_at  timestamptz NOT NULL DEFAULT now()
);

-- seed defaults
INSERT INTO app_config (key, value, scope, updated_by)
VALUES
('decision.mode', '"suggest"', 'global', 'seed'),
('decision.max_canary_hosts', '2', 'global', 'seed'),
('guardrails.never_block_labels', '["role:db","role:control-plane"]', 'global', 'seed'),
('correlator.rule_window_sec', '5', 'global', 'seed')
ON CONFLICT (key) DO NOTHING;

