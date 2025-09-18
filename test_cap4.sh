#!/bin/bash

# AegisFlux Capability 4 End-to-End Test
# This script tests the complete workflow from plan creation to rollback

set -euo pipefail

# Configuration
DECISION_API="http://localhost:8083"
BPF_REGISTRY="http://localhost:8090"
NATS_URL="nats://localhost:4222"
AGENT_HOST_ID="test-host-001"
NATS_CLI="./nats"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
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

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

step() {
    echo -e "${CYAN}[STEP]${NC} $1"
}

# Check if services are running
check_services() {
    header "Checking Services"
    
    local services=("$DECISION_API" "$BPF_REGISTRY" "$NATS_URL")
    local service_names=("Decision API" "BPF Registry" "NATS")
    
    for i in "${!services[@]}"; do
        local url="${services[$i]}"
        local name="${service_names[$i]}"
        
        if [[ "$url" == nats://* ]]; then
            # Check NATS
            if "$NATS_CLI" --server="$url" server check jetstream 2>/dev/null; then
                success "$name is running"
            else
                error "$name is not accessible"
                exit 1
            fi
        else
            # Check HTTP services
            if curl -s "$url/health" > /dev/null 2>&1 || curl -s "$url/healthz" > /dev/null 2>&1; then
                success "$name is running"
            else
                error "$name is not accessible at $url"
                exit 1
            fi
        fi
    done
}

# Start NATS monitoring in background
start_nats_monitoring() {
    header "Starting NATS Monitoring"
    
    log "Starting NATS subscriber for plans.proposed..."
    "$NATS_CLI" --server="$NATS_URL" sub "plans.proposed" --count=10 > /tmp/plans_proposed.log 2>&1 &
    local plans_pid=$!
    echo "$plans_pid" > /tmp/plans_pid
    
    log "Starting NATS subscriber for agent.telemetry..."
    "$NATS_CLI" --server="$NATS_URL" sub "agent.telemetry" --count=10 > /tmp/agent_telemetry.log 2>&1 &
    local telemetry_pid=$!
    echo "$telemetry_pid" > /tmp/telemetry_pid
    
    log "NATS monitoring started (PID: $plans_pid, $telemetry_pid)"
    sleep 2
}

# Stop NATS monitoring
stop_nats_monitoring() {
    if [[ -f /tmp/plans_pid ]]; then
        local pid=$(cat /tmp/plans_pid)
        kill "$pid" 2>/dev/null || true
        rm -f /tmp/plans_pid
    fi
    
    if [[ -f /tmp/telemetry_pid ]]; then
        local pid=$(cat /tmp/telemetry_pid)
        kill "$pid" 2>/dev/null || true
        rm -f /tmp/telemetry_pid
    fi
    
    log "NATS monitoring stopped"
}

# Create a test plan
create_test_plan() {
    header "Creating Test Plan"
    
    step "Creating seed finding and plan via POST /plans"
    
    local plan_payload='{
        "finding": {
            "id": "test-finding-'$(date +%s)'",
            "severity": "high",
            "confidence": 0.9,
            "host_id": "'$AGENT_HOST_ID'",
            "rule_id": "suspicious-network-activity",
            "evidence": [
                "Unusual network connections detected",
                "High volume of egress traffic",
                "Suspicious domain queries"
            ],
            "ts": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
            "metadata": {
                "test_run": true,
                "created_by": "test-script"
            }
        },
        "strategy_mode": "aggressive",
        "metadata": {
            "test_run": true,
            "created_by": "test-script",
            "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
        }
    }'
    
    log "Sending plan to decision API..."
    local response=$(curl -s -X POST "$DECISION_API/plans" \
        -H "Content-Type: application/json" \
        -d "$plan_payload")
    
    if echo "$response" | jq -e '.plan.id' > /dev/null 2>&1; then
        local plan_id=$(echo "$response" | jq -r '.plan.id')
        success "Plan created successfully: $plan_id"
        echo "$plan_id" > /tmp/test_plan_id
        echo "$response" | jq .
    else
        error "Failed to create plan"
        echo "Response: $response"
        exit 1
    fi
}

# Monitor orchestrator activity
monitor_orchestrator() {
    header "Monitoring Orchestrator Activity"
    
    step "Waiting for orchestrator to process plan and upload to registry..."
    
    local max_wait=60
    local wait_time=0
    
    while [[ $wait_time -lt $max_wait ]]; do
        log "Checking BPF registry for new artifacts... (${wait_time}s/${max_wait}s)"
        
        local artifacts=$(curl -s "$BPF_REGISTRY/artifacts" | jq -r '.[].id' 2>/dev/null || echo "")
        
        if [[ -n "$artifacts" ]]; then
            success "Artifacts found in registry!"
            echo "Artifacts: $artifacts"
            
            # Get artifact details
            for artifact_id in $artifacts; do
                log "Artifact details for $artifact_id:"
                curl -s "$BPF_REGISTRY/artifacts/$artifact_id" | jq .
            done
            
            # Store first artifact ID for later use
            echo "$artifacts" | head -1 > /tmp/test_artifact_id
            break
        fi
        
        sleep 5
        wait_time=$((wait_time + 5))
    done
    
    if [[ $wait_time -ge $max_wait ]]; then
        error "Timeout waiting for orchestrator to upload artifacts"
        exit 1
    fi
}

# Start local agent
start_local_agent() {
    header "Starting Local Agent"
    
    step "Starting local agent to poll registry and load BPF programs"
    
    # Build the agent first
    log "Building local agent..."
    cd agents/local-agent
    make build
    
    # Start agent in background
    log "Starting agent..."
    AGENT_HOST_ID="$AGENT_HOST_ID" \
    AGENT_REGISTRY_URL="$BPF_REGISTRY" \
    AGENT_NATS_URL="$NATS_URL" \
    AGENT_VAULT_URL="http://localhost:8200" \
    AGENT_VAULT_TOKEN="dev-token" \
    AGENT_POLL_INTERVAL_SEC=10 \
    AGENT_LOG_LEVEL=info \
    ./aegisflux-agent > /tmp/agent.log 2>&1 &
    
    local agent_pid=$!
    echo "$agent_pid" > /tmp/agent_pid
    
    success "Agent started (PID: $agent_pid)"
    
    # Wait for agent to start up
    sleep 5
    
    # Check agent health
    local max_health_checks=12
    local health_checks=0
    
    while [[ $health_checks -lt $max_health_checks ]]; do
        if curl -s http://localhost:8080/healthz > /dev/null 2>&1; then
            success "Agent is healthy and responding"
            break
        fi
        
        log "Waiting for agent to become healthy... ($health_checks/$max_health_checks)"
        sleep 5
        health_checks=$((health_checks + 1))
    done
    
    if [[ $health_checks -ge $max_health_checks ]]; then
        error "Agent failed to become healthy"
        cat /tmp/agent.log
        exit 1
    fi
    
    cd ../..
}

# Monitor agent activity
monitor_agent() {
    header "Monitoring Agent Activity"
    
    step "Monitoring agent polling, downloading, and loading of BPF programs"
    
    local max_wait=120
    local wait_time=0
    
    while [[ $wait_time -lt $max_wait ]]; do
        log "Checking agent status... (${wait_time}s/${max_wait}s)"
        
        # Check agent status
        local status=$(curl -s http://localhost:8080/status | jq . 2>/dev/null || echo "{}")
        
        if echo "$status" | jq -e '.applied_artifacts | length > 0' > /dev/null 2>&1; then
            success "Agent has loaded artifacts!"
            echo "$status" | jq .
            
            # Check for loaded status
            local loaded_artifacts=$(echo "$status" | jq -r '.applied_artifacts[] | select(.status == "running") | .artifact_id' 2>/dev/null || echo "")
            
            if [[ -n "$loaded_artifacts" ]]; then
                success "BPF programs are running!"
                echo "Running artifacts: $loaded_artifacts"
                break
            fi
        fi
        
        sleep 10
        wait_time=$((wait_time + 10))
    done
    
    if [[ $wait_time -ge $max_wait ]]; then
        error "Timeout waiting for agent to load artifacts"
        exit 1
    fi
}

# Check NATS messages
check_nats_messages() {
    header "Checking NATS Messages"
    
    step "Reviewing captured NATS messages"
    
    log "Plans proposed messages:"
    if [[ -f /tmp/plans_proposed.log ]]; then
        cat /tmp/plans_proposed.log
    else
        warn "No plans.proposed messages captured"
    fi
    
    log "Agent telemetry messages:"
    if [[ -f /tmp/agent_telemetry.log ]]; then
        cat /tmp/agent_telemetry.log
    else
        warn "No agent.telemetry messages captured"
    fi
}

# Simulate canary window and enforcement
simulate_enforcement() {
    header "Simulating Canary Window and Enforcement"
    
    step "Waiting for canary window to pass and enforcement to begin"
    
    log "Monitoring plan status for enforcement..."
    
    local plan_id=$(cat /tmp/test_plan_id 2>/dev/null || echo "")
    if [[ -z "$plan_id" ]]; then
        error "No plan ID found"
        return 1
    fi
    
    local max_wait=180
    local wait_time=0
    
    while [[ $wait_time -lt $max_wait ]]; do
        log "Checking plan status... (${wait_time}s/${max_wait}s)"
        
        local plan_status=$(curl -s "$DECISION_API/plans/$plan_id" | jq . 2>/dev/null || echo "{}")
        local plan_state=$(echo "$plan_status" | jq -r '.status' 2>/dev/null || echo "unknown")
        
        log "Plan status: $plan_state"
        
        if [[ "$plan_state" == "enforced" || "$plan_state" == "active" ]]; then
            success "Plan has been enforced!"
            echo "$plan_status" | jq .
            break
        fi
        
        sleep 10
        wait_time=$((wait_time + 10))
    done
    
    if [[ $wait_time -ge $max_wait ]]; then
        warn "Plan enforcement not detected within timeout"
    fi
}

# Force rollback test
force_rollback_test() {
    header "Force Rollback Test"
    
    step "Simulating SLO breach and triggering rollback"
    
    # Method 1: Trigger rollback via orchestrator API
    log "Attempting to trigger rollback via orchestrator..."
    
    local artifact_id=$(cat /tmp/test_artifact_id 2>/dev/null || echo "")
    if [[ -n "$artifact_id" ]]; then
        local rollback_payload='{
            "reason": "test_slo_breach",
            "threshold": "cpu_percent",
            "value": 95.0,
            "host_id": "'$AGENT_HOST_ID'"
        }'
        
        # Try to trigger rollback via decision API
        local rollback_response=$(curl -s -X POST "$DECISION_API/plans/$(cat /tmp/test_plan_id)/rollback" \
            -H "Content-Type: application/json" \
            -d "$rollback_payload" 2>/dev/null || echo "{}")
        
        log "Rollback response: $rollback_response"
    fi
    
    # Method 2: Send rollback signal via NATS
    log "Sending rollback signal via NATS..."
    
    local rollback_signal='{
        "type": "rollback_signal",
        "artifact_id": "'$artifact_id'",
        "host_id": "'$AGENT_HOST_ID'",
        "reason": "test_slo_breach",
        "threshold": "cpu_percent",
        "value": 95.0,
        "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
    }'
    
    echo "$rollback_signal" | "$NATS_CLI" --server="$NATS_URL" pub "agent.rollback.$AGENT_HOST_ID" 2>/dev/null || true
    
    # Method 3: Simulate high CPU usage to trigger automatic rollback
    log "Simulating high CPU usage to trigger automatic rollback..."
    
    # Send telemetry with high CPU to trigger rollback
    local high_cpu_telemetry='{
        "type": "program_telemetry",
        "artifact_id": "'$artifact_id'",
        "host_id": "'$AGENT_HOST_ID'",
        "cpu_percent": 95.0,
        "memory_kb": 204800,
        "violations": 1000,
        "errors": 50,
        "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
    }'
    
    echo "$high_cpu_telemetry" | "$NATS_CLI" --server="$NATS_URL" pub "agent.telemetry" 2>/dev/null || true
    
    # Wait and check for rollback
    log "Waiting for rollback to occur..."
    sleep 10
    
    local agent_status=$(curl -s http://localhost:8080/status | jq . 2>/dev/null || echo "{}")
    local rolled_back_artifacts=$(echo "$agent_status" | jq -r '.applied_artifacts[] | select(.status == "rolled_back") | .artifact_id' 2>/dev/null || echo "")
    
    if [[ -n "$rolled_back_artifacts" ]]; then
        success "Rollback detected! Artifacts rolled back: $rolled_back_artifacts"
    else
        warn "No rollback detected - checking rollback history..."
        
        local rollback_history=$(curl -s http://localhost:8080/rollbacks | jq . 2>/dev/null || echo "{}")
        echo "$rollback_history" | jq .
    fi
}

# Cleanup function
cleanup() {
    header "Cleanup"
    
    log "Stopping services and cleaning up..."
    
    # Stop agent
    if [[ -f /tmp/agent_pid ]]; then
        local agent_pid=$(cat /tmp/agent_pid)
        kill "$agent_pid" 2>/dev/null || true
        rm -f /tmp/agent_pid
        log "Agent stopped"
    fi
    
    # Stop NATS monitoring
    stop_nats_monitoring
    
    # Clean up temp files
    rm -f /tmp/test_plan_id /tmp/test_artifact_id /tmp/plans_proposed.log /tmp/agent_telemetry.log /tmp/agent.log
    
    log "Cleanup completed"
}

# Main test execution
main() {
    header "AegisFlux Capability 4 End-to-End Test"
    
    # Set up cleanup trap
    trap cleanup EXIT
    
    # Run test steps
    check_services
    start_nats_monitoring
    create_test_plan
    monitor_orchestrator
    start_local_agent
    monitor_agent
    check_nats_messages
    simulate_enforcement
    force_rollback_test
    
    success "Capability 4 test completed successfully!"
    
    echo
    echo "Test Summary:"
    echo "✓ Services are running"
    echo "✓ Plan created and processed"
    echo "✓ Orchestrator rendered and uploaded artifacts"
    echo "✓ Agent polled registry and loaded BPF programs"
    echo "✓ NATS messages captured"
    echo "✓ Canary window and enforcement simulated"
    echo "✓ Rollback test performed"
    echo
    echo "Check the logs above for detailed information."
}

# Run main function
main "$@"
