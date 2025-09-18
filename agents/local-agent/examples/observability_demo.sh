#!/bin/bash

# AegisFlux Agent Observability Demo
# This script demonstrates the observability features of the agent

set -euo pipefail

AGENT_URL="http://localhost:8080"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

header() {
    echo -e "${BLUE}=== $1 ===${NC}"
}

check_agent() {
    if ! curl -s "$AGENT_URL/healthz" > /dev/null; then
        error "Agent is not running at $AGENT_URL"
        error "Please start the agent first: make run-dev"
        exit 1
    fi
    log "Agent is running and healthy"
}

demo_health_check() {
    header "Health Check Demo"
    
    echo "Checking agent health..."
    curl -s "$AGENT_URL/healthz" | jq .
    echo
}

demo_status() {
    header "Status Information Demo"
    
    echo "Getting detailed status..."
    curl -s "$AGENT_URL/status" | jq .
    echo
}

demo_metrics() {
    header "Metrics Demo"
    
    echo "Getting metrics..."
    curl -s "$AGENT_URL/metrics" | jq .
    echo
}

demo_programs() {
    header "Program Information Demo"
    
    echo "Getting program details..."
    curl -s "$AGENT_URL/programs" | jq .
    echo
}

demo_telemetry() {
    header "Telemetry Data Demo"
    
    echo "Getting current telemetry..."
    curl -s "$AGENT_URL/telemetry" | jq .
    echo
}

demo_rollbacks() {
    header "Rollback History Demo"
    
    echo "Getting rollback history..."
    curl -s "$AGENT_URL/rollbacks" | jq .
    echo
}

demo_version() {
    header "Version Information Demo"
    
    echo "Getting version info..."
    curl -s "$AGENT_URL/version" | jq .
    echo
}

demo_monitoring() {
    header "Continuous Monitoring Demo"
    
    echo "Monitoring agent for 30 seconds..."
    echo "Press Ctrl+C to stop"
    
    for i in {1..30}; do
        echo -n "Health check $i: "
        if curl -s "$AGENT_URL/healthz" | jq -r '.status'; then
            echo -e "${GREEN}✓${NC}"
        else
            echo -e "${RED}✗${NC}"
        fi
        sleep 1
    done
    echo
}

demo_logging() {
    header "Structured Logging Demo"
    
    echo "The agent uses structured JSON logging with the following categories:"
    echo
    echo "Program Events:"
    echo "  - program_loaded: When a BPF program is loaded"
    echo "  - program_unloaded: When a BPF program is unloaded"
    echo "  - program_rolled_back: When a program is rolled back"
    echo "  - program_error: When a program encounters an error"
    echo
    echo "System Events:"
    echo "  - agent_started: When the agent starts"
    echo "  - agent_stopped: When the agent stops"
    echo "  - config_loaded: When configuration is loaded"
    echo "  - http_server_started: When HTTP server starts"
    echo
    echo "Telemetry Events:"
    echo "  - telemetry_sent: When telemetry is sent to NATS"
    echo "  - threshold_exceeded: When monitoring thresholds are exceeded"
    echo
    echo "To view logs in real-time:"
    echo "  sudo journalctl -u aegisflux-agent -f"
    echo "  # or for development:"
    echo "  tail -f /tmp/aegisflux-agent/agent.log"
    echo
}

demo_systemd() {
    header "Systemd Integration Demo"
    
    echo "The agent provides full systemd integration:"
    echo
    echo "Service Management:"
    echo "  sudo systemctl start aegisflux-agent"
    echo "  sudo systemctl stop aegisflux-agent"
    echo "  sudo systemctl restart aegisflux-agent"
    echo "  sudo systemctl status aegisflux-agent"
    echo
    echo "Log Viewing:"
    echo "  sudo journalctl -u aegisflux-agent -f"
    echo "  sudo journalctl -u aegisflux-agent --since '1 hour ago'"
    echo
    echo "Service Notifications:"
    echo "  - READY=1: Service is ready"
    echo "  - STOPPING=1: Service is stopping"
    echo "  - WATCHDOG=1: Watchdog ping"
    echo "  - STATUS=<message>: Custom status"
    echo
}

demo_alerting() {
    header "Alerting and Monitoring Demo"
    
    echo "You can set up alerts based on:"
    echo
    echo "Health Check Failures:"
    echo "  curl -f $AGENT_URL/healthz || alert 'Agent health check failed'"
    echo
    echo "High Error Rates:"
    echo "  ERRORS=\$(curl -s $AGENT_URL/metrics | jq '.gauges.total_errors')"
    echo "  if [ \"\$ERRORS\" -gt 10 ]; then alert 'High error rate detected'; fi"
    echo
    echo "Program Failures:"
    echo "  FAILED=\$(curl -s $AGENT_URL/metrics | jq '.counters.failed_programs')"
    echo "  if [ \"\$FAILED\" -gt 0 ]; then alert 'Program failures detected'; fi"
    echo
    echo "Rollback Events:"
    echo "  ROLLBACKS=\$(curl -s $AGENT_URL/rollbacks | jq '.total')"
    echo "  if [ \"\$ROLLBACKS\" -gt 0 ]; then alert 'Rollback events detected'; fi"
    echo
}

demo_integration() {
    header "Integration Examples"
    
    echo "Prometheus Scraping (via custom exporter):"
    echo "  curl -s $AGENT_URL/metrics | convert_to_prometheus_format"
    echo
    echo "Grafana Dashboard:"
    echo "  - Use HTTP API endpoints as data sources"
    echo "  - Create panels for health, metrics, and status"
    echo
    echo "ELK Stack Integration:"
    echo "  - Structured JSON logs work well with Elasticsearch"
    echo "  - Parse logs using JSON filter in Logstash"
    echo
    echo "Kubernetes Health Checks:"
    echo "  livenessProbe:"
    echo "    httpGet:"
    echo "      path: /healthz"
    echo "      port: 8080"
    echo "  readinessProbe:"
    echo "    httpGet:"
    echo "      path: /healthz"
    echo "      port: 8080"
    echo
}

main() {
    log "Starting AegisFlux Agent Observability Demo"
    echo
    
    check_agent
    
    demo_health_check
    demo_status
    demo_metrics
    demo_programs
    demo_telemetry
    demo_rollbacks
    demo_version
    demo_monitoring
    demo_logging
    demo_systemd
    demo_alerting
    demo_integration
    
    log "Observability demo completed!"
    echo
    echo "For more detailed information, see:"
    echo "  - docs/OBSERVABILITY.md"
    echo "  - README.md"
    echo "  - systemd/aegisflux-agent.service"
    echo
}

main "$@"
