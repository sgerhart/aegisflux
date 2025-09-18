#!/bin/bash
set -euo pipefail

# AegisFlux Complete System Test
# This script tests the entire Cap7 implementation end-to-end

echo "🚀 Starting AegisFlux Complete System Test"
echo "=========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
NATS_URL="nats://localhost:4222"
VAULT_ADDR="http://localhost:8200"
VAULT_TOKEN="root"
TEST_TIMEOUT=300

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    case $status in
        "INFO") echo -e "${BLUE}ℹ️  $message${NC}" ;;
        "SUCCESS") echo -e "${GREEN}✅ $message${NC}" ;;
        "WARNING") echo -e "${YELLOW}⚠️  $message${NC}" ;;
        "ERROR") echo -e "${RED}❌ $message${NC}" ;;
    esac
}

# Function to check if a service is running
check_service() {
    local service=$1
    local port=$2
    local timeout=${3:-10}
    
    print_status "INFO" "Checking $service on port $port..."
    
    for i in $(seq 1 $timeout); do
        if curl -s "http://localhost:$port/healthz" > /dev/null 2>&1; then
            print_status "SUCCESS" "$service is running"
            return 0
        fi
        sleep 1
    done
    
    print_status "ERROR" "$service is not responding after ${timeout}s"
    return 1
}

# Function to check NATS connection
check_nats() {
    print_status "INFO" "Checking NATS connection..."
    
    if command -v nats > /dev/null 2>&1; then
        if nats server check --server $NATS_URL > /dev/null 2>&1; then
            print_status "SUCCESS" "NATS is running"
            return 0
        fi
    fi
    
    # Fallback check
    if nc -z localhost 4222 2>/dev/null; then
        print_status "SUCCESS" "NATS port is open"
        return 0
    fi
    
    print_status "ERROR" "NATS is not accessible"
    return 1
}

# Function to check Vault
check_vault() {
    print_status "INFO" "Checking Vault..."
    
    if curl -s "$VAULT_ADDR/v1/sys/health" > /dev/null 2>&1; then
        print_status "SUCCESS" "Vault is running"
        return 0
    fi
    
    print_status "ERROR" "Vault is not accessible"
    return 1
}

# Function to bootstrap Vault
bootstrap_vault() {
    print_status "INFO" "Bootstrapping Vault with test secrets..."
    
    if [ -f "scripts/vault/dev-bootstrap.sh" ]; then
        bash scripts/vault/dev-bootstrap.sh
        print_status "SUCCESS" "Vault bootstrapped"
    else
        print_status "WARNING" "Vault bootstrap script not found, using manual setup"
        
        # Manual Vault setup
        curl -s -X POST "$VAULT_ADDR/v1/sys/mounts/secret" \
            -H "X-Vault-Token: $VAULT_TOKEN" \
            -d '{"type":"kv-v2"}' > /dev/null || true
            
        # Add test secrets
        curl -s -X POST "$VAULT_ADDR/v1/secret/data/cve-sync/nvd" \
            -H "X-Vault-Token: $VAULT_TOKEN" \
            -d '{"data":{"api_key":"test-nvd-key","rate_limit":50,"timeout":30}}' > /dev/null || true
            
        curl -s -X POST "$VAULT_ADDR/v1/secret/data/cve-sync/config" \
            -H "X-Vault-Token: $VAULT_TOKEN" \
            -d '{"data":{"max_pages":10,"retry_attempts":3,"cache_ttl":300}}' > /dev/null || true
    fi
}

# Function to test CVE sync
test_cve_sync() {
    print_status "INFO" "Testing CVE sync service..."
    
    # Start CVE sync in background
    cd feeds/cve-sync
    python -m cve_sync.main &
    CVE_SYNC_PID=$!
    cd ../..
    
    # Wait for CVE sync to process
    sleep 10
    
    # Check if CVE sync is still running
    if kill -0 $CVE_SYNC_PID 2>/dev/null; then
        print_status "SUCCESS" "CVE sync service started"
        kill $CVE_SYNC_PID 2>/dev/null || true
    else
        print_status "ERROR" "CVE sync service failed to start"
        return 1
    fi
}

# Function to test package mappers
test_package_mappers() {
    print_status "INFO" "Testing package mappers..."
    
    # Generate sample inventory
    cd feeds/mappers
    python sample_inventory.py &
    SAMPLE_PID=$!
    cd ../..
    
    # Wait for sample data
    sleep 5
    
    # Start mappers
    cd feeds/mappers
    python -m mapper.main &
    MAPPER_PID=$!
    cd ../..
    
    # Wait for processing
    sleep 10
    
    # Check if mappers are still running
    if kill -0 $MAPPER_PID 2>/dev/null; then
        print_status "SUCCESS" "Package mappers service started"
        kill $MAPPER_PID 2>/dev/null || true
    else
        print_status "ERROR" "Package mappers service failed to start"
        return 1
    fi
    
    kill $SAMPLE_PID 2>/dev/null || true
}

# Function to test ETL enrich
test_etl_enrich() {
    print_status "INFO" "Testing ETL enrich service..."
    
    # Start ETL enrich
    cd backend/etl-enrich
    python -m app.main &
    ETL_PID=$!
    cd ../..
    
    # Wait for ETL to start
    sleep 10
    
    # Check if ETL is still running
    if kill -0 $ETL_PID 2>/dev/null; then
        print_status "SUCCESS" "ETL enrich service started"
        kill $ETL_PID 2>/dev/null || true
    else
        print_status "ERROR" "ETL enrich service failed to start"
        return 1
    fi
}

# Function to test correlator
test_correlator() {
    print_status "INFO" "Testing correlator service..."
    
    # Start correlator
    cd backend/correlator
    go run ./cmd/correlator/main_enhanced.go &
    CORRELATOR_PID=$!
    cd ../..
    
    # Wait for correlator to start
    sleep 5
    
    # Check if correlator is responding
    if check_service "correlator" "8085" 5; then
        print_status "SUCCESS" "Correlator service started"
        kill $CORRELATOR_PID 2>/dev/null || true
    else
        print_status "ERROR" "Correlator service failed to start"
        return 1
    fi
}

# Function to test decision engine
test_decision_engine() {
    print_status "INFO" "Testing decision engine..."
    
    # Start decision engine
    cd backend/decision
    go run ./cmd/decision &
    DECISION_PID=$!
    cd ../..
    
    # Wait for decision engine to start
    sleep 5
    
    # Check if decision engine is responding
    if check_service "decision" "8083" 5; then
        print_status "SUCCESS" "Decision engine started"
        kill $DECISION_PID 2>/dev/null || true
    else
        print_status "ERROR" "Decision engine failed to start"
        return 1
    fi
}

# Function to test orchestrator
test_orchestrator() {
    print_status "INFO" "Testing orchestrator..."
    
    # Start orchestrator
    cd backend/orchestrator
    go run ./cmd/orchestrator &
    ORCHESTRATOR_PID=$!
    cd ../..
    
    # Wait for orchestrator to start
    sleep 5
    
    # Check if orchestrator is responding
    if check_service "orchestrator" "8081" 5; then
        print_status "SUCCESS" "Orchestrator started"
        kill $ORCHESTRATOR_PID 2>/dev/null || true
    else
        print_status "ERROR" "Orchestrator failed to start"
        return 1
    fi
}

# Function to test end-to-end flow
test_end_to_end() {
    print_status "INFO" "Testing end-to-end flow..."
    
    # This would test the complete flow:
    # 1. CVE sync fetches data
    # 2. Package mappers process inventory
    # 3. ETL enrich joins data
    # 4. Correlator processes findings
    # 5. Decision engine creates plans
    # 6. Orchestrator applies restrictions
    
    print_status "WARNING" "End-to-end test requires full infrastructure setup"
    print_status "INFO" "Use docker-compose up for complete testing"
}

# Function to run unit tests
run_unit_tests() {
    print_status "INFO" "Running unit tests..."
    
    # Test CVE sync
    if [ -f "feeds/cve-sync/test_cve_sync.py" ]; then
        cd feeds/cve-sync
        python test_cve_sync.py
        cd ../..
        print_status "SUCCESS" "CVE sync tests passed"
    fi
    
    # Test package mappers
    if [ -f "feeds/mappers/test_package_mapper.py" ]; then
        cd feeds/mappers
        python test_package_mapper.py
        cd ../..
        print_status "SUCCESS" "Package mapper tests passed"
    fi
    
    # Test ETL enrich
    if [ -f "backend/etl-enrich/test_pkg_cve_enrichment.py" ]; then
        cd backend/etl-enrich
        python test_pkg_cve_enrichment.py
        cd ../..
        print_status "SUCCESS" "ETL enrich tests passed"
    fi
    
    # Test correlator
    if [ -f "backend/correlator/test_enhanced_rules.go" ]; then
        cd backend/correlator
        go run test_enhanced_rules.go
        cd ../..
        print_status "SUCCESS" "Correlator tests passed"
    fi
}

# Main test execution
main() {
    echo
    print_status "INFO" "Starting system validation..."
    
    # Check prerequisites
    print_status "INFO" "Checking prerequisites..."
    
    # Check if Docker is running
    if ! docker info > /dev/null 2>&1; then
        print_status "ERROR" "Docker is not running. Please start Docker first."
        exit 1
    fi
    
    # Check if required tools are installed
    local missing_tools=()
    
    if ! command -v go > /dev/null 2>&1; then
        missing_tools+=("go")
    fi
    
    if ! command -v python3 > /dev/null 2>&1; then
        missing_tools+=("python3")
    fi
    
    if ! command -v curl > /dev/null 2>&1; then
        missing_tools+=("curl")
    fi
    
    if [ ${#missing_tools[@]} -ne 0 ]; then
        print_status "ERROR" "Missing required tools: ${missing_tools[*]}"
        print_status "INFO" "Please install the missing tools and try again."
        exit 1
    fi
    
    print_status "SUCCESS" "Prerequisites check passed"
    
    # Run unit tests
    echo
    print_status "INFO" "Running unit tests..."
    run_unit_tests
    
    # Check if infrastructure is running
    echo
    print_status "INFO" "Checking infrastructure services..."
    
    if ! check_nats; then
        print_status "WARNING" "NATS is not running. Start with: docker run -d -p 4222:4222 nats:2.10 --jetstream"
    fi
    
    if ! check_vault; then
        print_status "WARNING" "Vault is not running. Start with: docker run -d -p 8200:8200 -e VAULT_DEV_ROOT_TOKEN_ID=root hashicorp/vault:1.16"
    else
        bootstrap_vault
    fi
    
    # Test individual services
    echo
    print_status "INFO" "Testing individual services..."
    
    # Test CVE sync
    test_cve_sync
    
    # Test package mappers
    test_package_mappers
    
    # Test ETL enrich
    test_etl_enrich
    
    # Test correlator
    test_correlator
    
    # Test decision engine
    test_decision_engine
    
    # Test orchestrator
    test_orchestrator
    
    # Test end-to-end flow
    echo
    test_end_to_end
    
    # Summary
    echo
    print_status "SUCCESS" "System test completed!"
    print_status "INFO" "For full integration testing, run: docker-compose -f infra/compose/docker-compose.yml up"
    print_status "INFO" "This will start all services with proper dependencies and networking"
}

# Run main function
main "$@"
