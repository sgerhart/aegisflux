# ETL Enrichment Service

A Python-based ETL service that consumes raw events from NATS, enriches them with additional metadata, and publishes them to both NATS and database systems.

## Features

- **NATS Consumer**: Subscribes to `events.raw` with queue group for load balancing
- **Event Enrichment**: Adds environment and reverse DNS information
- **Dual Publishing**: Publishes enriched events to `events.enriched` subject
- **Database Writers**: Stores events in TimescaleDB and Neo4j
- **Backpressure Handling**: Queue-based processing with configurable batch sizes
- **Retry Logic**: Automatic retry with exponential backoff
- **Health Monitoring**: Built-in health checks and metrics

## Architecture

```
Raw Events (NATS) → Consumer → Enrichment → Publishers
                                        ├── NATS (enriched events)
                                        ├── TimescaleDB (time-series)
                                        └── Neo4j (graph relationships)
```

## Project Structure

```
backend/etl-enrich/
├── app/
│   ├── consumer.py          # NATS consumer with backpressure
│   ├── enrich.py            # Event enrichment logic
│   ├── publish.py           # Enriched event publisher
│   ├── config.py            # Configuration management
│   └── writers/
│       ├── timescale.py     # TimescaleDB writer
│       └── neo4j.py         # Neo4j writer
├── tests/
│   ├── conftest.py          # Pytest fixtures
│   ├── test_enrich.py       # Enrichment tests
│   └── test_writers.py      # Database writer tests
├── requirements.txt         # Python dependencies
├── Dockerfile              # Container definition
└── README.md               # This file
```

## Environment Variables

### Required
- `NATS_URL`: NATS server URL (default: `nats://localhost:4222`)

### Database Configuration
- `PG_HOST`: PostgreSQL/TimescaleDB host (default: `localhost`)
- `PG_PORT`: PostgreSQL port (default: `5432`)
- `PG_DB`: Database name (default: `aegisflux`)
- `PG_USER`: Database user (default: `postgres`)
- `PG_PASSWORD`: Database password (default: `password`)

- `NEO4J_URI`: Neo4j URI (default: `bolt://localhost:7687`)
- `NEO4J_USER`: Neo4j username (default: `neo4j`)
- `NEO4J_PASSWORD`: Neo4j password (default: `password`)

### Application Configuration
- `AF_ENV`: Environment name (default: `dev`)
- `AF_FAKE_RDNS`: Enable fake reverse DNS generation (default: `false`)

### Processing Configuration
- `MAX_BATCH_SIZE`: Maximum batch size for processing (default: `100`)
- `PROCESSING_TIMEOUT`: Timeout for batch processing in seconds (default: `30`)

## Usage

### Local Development

```bash
# Install dependencies
pip install -r requirements.txt

# Set environment variables
export NATS_URL=nats://localhost:4222
export PG_HOST=localhost
export NEO4J_URI=bolt://localhost:7687
export AF_ENV=dev
export AF_FAKE_RDNS=true

# Run the service
python -m app.consumer
```

### Docker

```bash
# Build the image
docker build -t aegisflux-etl-enrich .

# Run with environment variables
docker run -e NATS_URL=nats://host.docker.internal:4222 \
           -e PG_HOST=host.docker.internal \
           -e NEO4J_URI=bolt://host.docker.internal:7687 \
           -e AF_ENV=prod \
           aegisflux-etl-enrich
```

### Docker Compose

```yaml
etl-enrich:
  build: .
  environment:
    - NATS_URL=nats://nats:4222
    - PG_HOST=timescaledb
    - NEO4J_URI=bolt://neo4j:7687
    - AF_ENV=prod
    - AF_FAKE_RDNS=true
  depends_on:
    - nats
    - timescaledb
    - neo4j
```

## Event Enrichment

The service enriches events with the following fields:

- `env`: Environment name from `AF_ENV`
- `rdns`: Fake reverse DNS (if `AF_FAKE_RDNS=true`)
- `metadata.enriched_at`: Timestamp of enrichment
- `metadata.enrichment_version`: Version of enrichment logic

### Fake Reverse DNS

When `AF_FAKE_RDNS=true`, the service generates fake reverse DNS names:
- For IP addresses: `host-{last_octet}.local`
- For hostnames: `host-{last_part}.local`
- Example: `192.168.1.100` → `host-100.local`

## Database Schemas

### TimescaleDB

```sql
CREATE TABLE events (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    source TEXT NOT NULL,
    timestamp BIGINT NOT NULL,
    env TEXT,
    rdns TEXT,
    metadata JSONB,
    payload BYTEA,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

SELECT create_hypertable('events', 'created_at');
```

### Neo4j

The service creates the following node types and relationships:

- `Event` nodes with event data
- `Host` nodes with host information
- `User` nodes for user context
- `Process` nodes for process information
- `Container` nodes for container context

Relationships:
- `(Host)-[:GENERATED]->(Event)`
- `(User)-[:EXECUTED]->(Event)`
- `(Process)-[:EXECUTED]->(Event)`
- `(Container)-[:GENERATED]->(Event)`

## Testing

```bash
# Run all tests
pytest

# Run with coverage
pytest --cov=app

# Run specific test file
pytest tests/test_enrich.py

# Run with verbose output
pytest -v
```

## Monitoring

The service provides the following monitoring capabilities:

- **Health Checks**: Built-in health check endpoints
- **Metrics**: Prometheus-compatible metrics
- **Logging**: Structured JSON logging
- **Retry Logic**: Automatic retry with exponential backoff

## Performance

- **Batch Processing**: Configurable batch sizes for optimal throughput
- **Backpressure**: Queue-based processing prevents memory issues
- **Connection Pooling**: Efficient database connection management
- **Async Processing**: Non-blocking I/O for high concurrency

## Error Handling

- **Retry Logic**: Automatic retry with exponential backoff
- **Dead Letter Queue**: Failed events are logged for manual review
- **Circuit Breaker**: Prevents cascading failures
- **Graceful Shutdown**: Proper cleanup on service termination




