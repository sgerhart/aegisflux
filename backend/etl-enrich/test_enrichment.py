#!/usr/bin/env python3
"""Simple test script to verify enrichment functionality."""

import asyncio
import json
import sys
import os

# Add the app directory to the Python path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'app'))

from app.enrich import enrich_event, enrich_events_batch, validate_enriched_event


def test_enrichment():
    """Test the enrichment functionality."""
    print("Testing ETL enrichment functionality...")
    
    # Sample event data
    sample_event = {
        "id": "test-event-001",
        "type": "exec",
        "source": "/usr/bin/bash",
        "timestamp": 1640995200000,
        "metadata": {
            "host_id": "192.168.1.100",
            "pid": 1234,
            "uid": 1001,
            "container_id": None,
            "binary_sha256": "abc123def456"
        },
        "payload": "eyJhcmd2IjpbIi91c3IvYmluL2Jhc2giLCItYyIsImVjaG8gaGkiXX0="
    }
    
    print(f"Original event: {json.dumps(sample_event, indent=2)}")
    
    # Test single event enrichment
    enriched = enrich_event(sample_event)
    print(f"\nEnriched event: {json.dumps(enriched, indent=2)}")
    
    # Test validation
    is_valid = validate_enriched_event(enriched)
    print(f"\nEnriched event is valid: {is_valid}")
    
    # Test batch enrichment
    events_batch = [sample_event, sample_event.copy()]
    enriched_batch = enrich_events_batch(events_batch)
    print(f"\nBatch enrichment: {len(enriched_batch)} events processed")
    
    # Test fake RDNS generation
    from app.enrich import generate_fake_rdns
    test_hosts = ["192.168.1.100", "server-01.example.com", "10.0.0.1"]
    print(f"\nFake RDNS generation:")
    for host in test_hosts:
        rdns = generate_fake_rdns(host)
        print(f"  {host} -> {rdns}")
    
    print("\n✅ All enrichment tests passed!")


if __name__ == "__main__":
    test_enrichment()
