# correlator (scaffold)
Consumes `etl.enriched` and runtime findings, applies rules/temporal joins, emits **Findings** on `correlator.findings`.
Also exposes HTTP API to list/add rules and hot-reload from `rules.d/`.
