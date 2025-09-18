# ETL Enrich Service

Consumes telemetry, CVE data, and package CVE mappings to build enriched documents for the correlator. Emits enriched records to `etl.enriched`.

## Features

- **Multi-Source Consumption**: Processes raw events, CVE updates, and package CVE mappings
- **CVE Data Caching**: Maintains in-memory cache of CVE documents from `feeds.cve.updates`
- **Package CVE Join Logic**: Joins package CVE mappings with full CVE data
- **Exploitability Scoring**: Calculates comprehensive exploitability scores for CVE candidates
- **Risk Level Assessment**: Determines risk levels based on exploitability scores
- **Enriched Record Publishing**: Publishes `pkg_cve_enriched` records to `etl.enriched`

## Architecture

### Input Sources
- **`events.raw`**: Raw telemetry events from agents
- **`feeds.cve.updates`**: CVE data from CVE sync service
- **`feeds.pkg.cve`**: Package CVE mappings from package mappers

### Output
- **`etl.enriched`**: Enriched records for downstream processing
- **`events.enriched`**: Enriched telemetry events (legacy)

## Package CVE Enrichment

### Join Logic
1. **CVE Data Caching**: Stores CVE documents from `feeds.cve.updates`
2. **Package CVE Processing**: Processes package CVE mappings from `feeds.pkg.cve`
3. **Immediate Processing**: If CVE data is available, processes immediately
4. **Deferred Processing**: If CVE data is not available, stores for later processing
5. **Enrichment**: Joins package CVE data with full CVE documents

### Exploitability Scoring

The exploitability score combines multiple factors:

1. **Package Matching Score** (40% weight)
   - Base score from package mapper
   - Reflects accuracy of package-to-CVE matching

2. **CVSS Score Influence** (30% weight)
   - Normalized CVSS v2/v3/v3.1 scores
   - Higher CVSS scores increase exploitability

3. **Severity Influence** (up to 30% weight)
   - Critical: 30% bonus
   - High: 20% bonus
   - Medium: 10% bonus
   - Low: 5% bonus

4. **CWE Risk Assessment** (10% bonus)
   - High-risk CWE patterns (SQL injection, XSS, etc.)
   - Common vulnerability types

5. **Documentation Quality** (5% bonus)
   - Well-documented CVEs with multiple references
   - Indicates active exploitation

6. **Recency Bonus** (5% bonus)
   - Recent CVEs (< 30 days old)
   - Higher likelihood of active exploitation

### Risk Level Determination

- **CRITICAL**: Exploitability score ≥ 0.8
- **HIGH**: Exploitability score ≥ 0.6
- **MEDIUM**: Exploitability score ≥ 0.4
- **LOW**: Exploitability score ≥ 0.2
- **MINIMAL**: Exploitability score < 0.2

## Message Formats

### Input: `feeds.cve.updates`
```json
{
  "cve_id": "CVE-2023-1234",
  "published": "2023-01-01T00:00:00Z",
  "descriptions": [...],
  "cvss": {...},
  "cwe": {...},
  "affected_products": [...],
  "references": [...]
}
```

### Input: `feeds.pkg.cve`
```json
{
  "host_id": "web-01",
  "package": {
    "name": "openssl",
    "version": "3.0.2-0ubuntu1.6",
    "distro": "ubuntu"
  },
  "candidates": [
    {
      "cve_id": "CVE-2023-1234",
      "score": 0.875,
      "cvss_score": 7.5,
      "severity": "HIGH"
    }
  ]
}
```

### Output: `etl.enriched`
```json
{
  "record_type": "pkg_cve_enriched",
  "timestamp": "2023-01-01T00:00:00Z",
  "host_id": "web-01",
  "package": {
    "name": "openssl",
    "version": "3.0.2-0ubuntu1.6",
    "distro": "ubuntu"
  },
  "cve_candidate": {
    "cve_id": "CVE-2023-1234",
    "score": 0.875,
    "cvss_score": 7.5,
    "severity": "HIGH"
  },
  "cve_data": {
    "cve_id": "CVE-2023-1234",
    "published": "2023-01-01T00:00:00Z",
    "descriptions": [...],
    "cvss": {...},
    "cwe": {...},
    "affected_products": [...],
    "references": [...]
  },
  "enrichment": {
    "exploitability_score": 0.875,
    "risk_level": "CRITICAL",
    "enrichment_timestamp": "2023-01-01T00:00:00Z",
    "enrichment_version": "1.0"
  },
  "metadata": {
    "source": "etl-enrich",
    "enrichment_pipeline": "pkg_cve_join"
  }
}
```

## Usage

### Basic Usage
```bash
# Start the ETL enrich service
python -m app.main

# With custom configuration
NATS_URL=nats://nats:4222 python -m app.main
```

### Testing
```bash
# Run unit tests
python test_pkg_cve_enrichment.py

# Generate sample data
python sample_pkg_cve_data.py
```

### Docker Usage
```bash
# Build the image
docker build -t aegisflux/etl-enrich .

# Run with environment variables
docker run -e NATS_URL=nats://nats:4222 aegisflux/etl-enrich
```

## Configuration

### Environment Variables
- `NATS_URL`: NATS server URL (default: `nats://localhost:4222`)
- `AF_ENV`: Environment name (default: `dev`)
- `AF_FAKE_RDNS`: Generate fake reverse DNS (default: `true`)

### Database Configuration
- `PG_HOST`: TimescaleDB host (default: `localhost`)
- `PG_PORT`: TimescaleDB port (default: `5432`)
- `PG_DB`: TimescaleDB database (default: `aegisflux`)
- `NEO4J_URI`: Neo4j connection URI (default: `bolt://localhost:7687`)

## Performance

### Caching Strategy
- **CVE Cache**: In-memory cache of CVE documents
- **Package CVE Cache**: In-memory cache of package CVE mappings
- **Deferred Processing**: Handles out-of-order message delivery

### Scalability
- **Async Processing**: Non-blocking message processing
- **Semaphore Control**: Configurable concurrency limits
- **Timeout Handling**: Per-message timeout protection

## Monitoring

### Logging
- **Structured Logging**: JSON-formatted log messages
- **Performance Metrics**: Processing times and throughput
- **Error Tracking**: Comprehensive error logging

### Metrics
- **Messages Processed**: Per subject message counts
- **Cache Hit Rates**: CVE and package CVE cache performance
- **Enrichment Latency**: Time to process and enrich records
- **Error Rates**: Failed processing attempts

## Integration

### Upstream Services
- **CVE Sync**: Provides CVE data via `feeds.cve.updates`
- **Package Mappers**: Provides package CVE mappings via `feeds.pkg.cve`
- **Agents**: Provides raw telemetry via `events.raw`

### Downstream Services
- **Correlator**: Consumes enriched records for correlation
- **Decision Engine**: Uses enriched records for risk assessment
- **Alerting**: Triggers alerts based on enriched data

## Development

### Testing
- **Unit Tests**: Comprehensive test coverage for all functions
- **Integration Tests**: End-to-end testing with sample data
- **Performance Tests**: Load testing and benchmarking

### Extensibility
- **Plugin Architecture**: Support for custom enrichment rules
- **Configurable Scoring**: Adjustable exploitability scoring weights
- **Custom Risk Levels**: Configurable risk level thresholds

## Security Considerations

### Data Handling
- **Input Validation**: Validates all incoming message formats
- **Error Handling**: Graceful degradation on processing errors
- **Memory Management**: Efficient cache management and cleanup

### Privacy
- **Data Minimization**: Only processes necessary fields
- **Audit Logging**: Comprehensive audit trail for data processing
- **Access Control**: Secure access to enriched data

## Troubleshooting

### Common Issues
- **Memory Usage**: Monitor CVE cache size and growth
- **Processing Delays**: Check for message queue backlogs
- **Database Connections**: Verify TimescaleDB and Neo4j connectivity

### Debug Mode
```bash
# Enable debug logging
export LOG_LEVEL=DEBUG
python -m app.main
```

### Health Checks
- **NATS Connectivity**: Verify NATS server connection
- **Database Health**: Check TimescaleDB and Neo4j status
- **Cache Status**: Monitor cache hit rates and memory usage
