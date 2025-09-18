#!/bin/bash

# Example usage of the seg_maps API
# Make sure the orchestrator is running on localhost:8081

echo "Testing seg_maps API..."

# Test valid MapSnapshot
echo "1. Testing valid MapSnapshot..."
curl -X POST http://localhost:8081/seg/maps \
  -H "Content-Type: application/json" \
  -d '{
    "version": 1,
    "service_id": 123,
    "ttl_seconds": 300,
    "edges": [
      {
        "dst_cidr": "192.168.1.0/24",
        "proto": "tcp",
        "port": 80
      }
    ],
    "allow_cidrs": [
      {
        "cidr": "10.0.0.0/8",
        "proto": "tcp",
        "port": 443
      }
    ],
    "meta": {
      "description": "Test segmentation map"
    }
  }' \
  -w "\nHTTP Status: %{http_code}\n\n"

# Test with target hosts
echo "2. Testing with target hosts..."
curl -X POST "http://localhost:8081/seg/maps?target_host=host1&target_host=host2" \
  -H "Content-Type: application/json" \
  -d '{
    "version": 1,
    "service_id": 456,
    "ttl_seconds": 600
  }' \
  -w "\nHTTP Status: %{http_code}\n\n"

# Test invalid data (missing required fields)
echo "3. Testing invalid data..."
curl -X POST http://localhost:8081/seg/maps \
  -H "Content-Type: application/json" \
  -d '{
    "version": "invalid",
    "service_id": 789
  }' \
  -w "\nHTTP Status: %{http_code}\n\n"

# Test promote endpoint
echo "4. Testing promote endpoint..."
curl -X POST http://localhost:8081/seg/maps/promote \
  -H "Content-Type: application/json" \
  -d '{
    "service_id": 123,
    "action": "promote"
  }' \
  -w "\nHTTP Status: %{http_code}\n\n"

# Test rollback endpoint
echo "5. Testing rollback endpoint..."
curl -X POST http://localhost:8081/seg/maps/rollback \
  -H "Content-Type: application/json" \
  -d '{
    "service_id": 123,
    "action": "rollback"
  }' \
  -w "\nHTTP Status: %{http_code}\n\n"

echo "API testing complete!"
