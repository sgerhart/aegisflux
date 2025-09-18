# Cap 7 — Feeds, ETL, and Correlator (Scaffold)
Generated: 2025-09-18T15:54:00.978073Z

This scaffold adds:
- **feeds/cve-sync** (Python): NVD fetch → `feeds.cve.updates`
- **feeds/mappers** (Python): inventory → candidate CVEs → `feeds.pkg.cve`
- **backend/etl-enrich** (Python): join & enrich → `etl.enriched`
- **backend/correlator** (Go): rules + temporal joins → `correlator.findings`
- **schemas/finding.json**
- **docker-compose.yml** for the Cap 7 path

## Run dev stack
```bash
docker compose up -d
# (optionally) publish a sample package inventory:
nats pub inventory.packages '{"host_id":"web-01","package":"openssl","version":"1.1.1"}' -s nats://localhost:4222
# watch topics
nats sub feeds.cve.updates -s nats://localhost:4222
nats sub feeds.pkg.cve -s nats://localhost:4222
nats sub etl.enriched -s nats://localhost:4222
nats sub correlator.findings -s nats://localhost:4222
```
