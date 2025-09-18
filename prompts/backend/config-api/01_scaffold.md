Scaffold a Go 1.21 service at backend/config-api with:
- POSTGRES envs (PG_HOST, PG_PORT, PG_USER, PG_PASS, PG_DB)
- /healthz, /readyz
- GET /config, GET /config/{key}, PUT /config/{key}
- Publish to NATS subject "config.changed" on writes
- Use table app_config (DDL already in data/timeseries/init.sql)
