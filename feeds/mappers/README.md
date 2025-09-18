# Package Mappers

Translates host package inventories to likely CVE matches using real package matching algorithms and heuristic scoring.

## Features

- **Real Package Matching**: Matches packages by name, epoch:version:release, distro, and architecture
- **Heuristic Scoring**: Ranks CVE candidates using multiple scoring factors
- **Version Range Checking**: Validates package versions against CVE affected product ranges
- **Distro-Specific Scoring**: Enhanced scoring for distribution-specific CVEs
- **CVSS Integration**: Incorporates CVSS scores into candidate ranking
- **CVE Cache**: Maintains in-memory cache of CVE data from `feeds.cve.updates`
- **Comprehensive Logging**: Detailed logging for monitoring and debugging

## Architecture

### Input: `inventory.packages`
Consumes package inventory messages from agents containing:
- Host identification
- Distribution information
- Package details (name, version, epoch, release, architecture)

### Output: `feeds.pkg.cve`
Publishes ranked CVE candidates with:
- Package information
- CVE candidates with scores and reasons
- CVSS scores and severity levels
- Timestamp and metadata

## Package Matching

### Name Normalization
- Removes common prefixes (`lib`, `python3-`, `node-`, etc.)
- Handles package aliases and variations
- Supports pattern matching for related packages

### Version Parsing
- Parses `epoch:version:release` format
- Handles complex version strings from different distributions
- Supports version range comparisons

### Vulnerability Checking
- Validates package versions against CVE affected product ranges
- Checks `version_start_including`, `version_end_including`
- Checks `version_start_excluding`, `version_end_excluding`
- Supports wildcard and range specifications

## Heuristic Scoring

### Scoring Factors
1. **Package Name Match** (0.3 points)
   - Exact name matches in CPE names
   - Normalized name matches
   - Pattern-based matches

2. **Version Vulnerability** (0.5 points)
   - Package version falls within CVE affected range
   - Critical for accurate vulnerability assessment

3. **CVSS Score Influence** (0.2 points)
   - Normalized CVSS v2/v3/v3.1 scores
   - Higher CVSS scores increase candidate ranking

4. **Distribution-Specific** (0.1 points)
   - Bonus for distro-specific CVE matches
   - Ubuntu/Debian vs RHEL/CentOS specific scoring

### Final Score
- Range: 0.0 to 1.0
- Only candidates with score > 0.1 are included
- Top 10 candidates per package are published

## Usage

### Basic Usage
```bash
# Start the package mapper
python -m mapper.main

# With custom NATS URL
NATS_URL=nats://nats:4222 python -m mapper.main
```

### Testing
```bash
# Run unit tests
python test_package_mapper.py

# Generate sample inventory
python sample_inventory.py
```

### Docker Usage
```bash
# Build the image
docker build -t aegisflux/package-mapper .

# Run with environment variables
docker run -e NATS_URL=nats://nats:4222 aegisflux/package-mapper
```

## Configuration

### Environment Variables
- `NATS_URL`: NATS server URL (default: `nats://localhost:4222`)

### Dependencies
- `nats-py`: NATS client for messaging
- Standard Python libraries for version parsing and matching

## Message Formats

### Input: `inventory.packages`
```json
{
  "host_id": "web-01",
  "timestamp": "2023-01-01T00:00:00.000Z",
  "distro": {
    "name": "ubuntu",
    "version": "22.04",
    "codename": "jammy"
  },
  "packages": [
    {
      "name": "openssl",
      "version": "3.0.2-0ubuntu1.6",
      "epoch": "",
      "release": "0ubuntu1.6",
      "arch": "amd64"
    }
  ],
  "metadata": {
    "agent_version": "1.0.0",
    "scan_duration_ms": 2500,
    "total_packages": 1
  }
}
```

### Output: `feeds.pkg.cve`
```json
{
  "host_id": "web-01",
  "package": {
    "name": "openssl",
    "version": "3.0.2-0ubuntu1.6",
    "epoch": "",
    "release": "0ubuntu1.6",
    "arch": "amd64",
    "distro": "ubuntu",
    "distro_version": "22.04"
  },
  "candidates": [
    {
      "cve_id": "CVE-2023-1234",
      "score": 0.875,
      "reason": "Package openssl version 3.0.2-0ubuntu1.6 matches affected products (CVSS: 7.5)",
      "cvss_score": 7.5,
      "severity": "HIGH"
    }
  ],
  "timestamp": "2023-01-01T00:00:00.000Z",
  "total_candidates": 1
}
```

## Integration

### CVE Data Source
- Subscribes to `feeds.cve.updates` for CVE data
- Maintains in-memory cache of CVE information
- Updates cache as new CVEs are published

### Downstream Consumers
- Correlator service for event correlation
- Decision engine for risk assessment
- Alerting systems for vulnerability notifications

## Performance

### Caching
- In-memory CVE cache for fast lookups
- Package pattern cache for efficient matching
- Configurable cache TTL and size limits

### Scalability
- Async processing for high throughput
- Batch processing of package inventories
- Efficient version comparison algorithms

## Monitoring

### Logging
- Structured logging with package and CVE details
- Performance metrics for processing times
- Error handling and recovery logging

### Metrics
- Packages processed per second
- CVE candidates generated per package
- Cache hit rates and memory usage
- Error rates and failure modes

## Development

### Testing
- Unit tests for all matching algorithms
- Integration tests with sample data
- Performance tests for scalability

### Extensibility
- Plugin architecture for custom matching rules
- Configurable scoring weights
- Support for additional package formats

## Security Considerations

### Input Validation
- Validates all input data formats
- Sanitizes package names and versions
- Handles malformed CVE data gracefully

### Error Handling
- Graceful degradation on errors
- Comprehensive error logging
- Recovery from transient failures
