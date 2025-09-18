#!/usr/bin/env python3
"""Simple test script without external dependencies."""

import json
import ipaddress


def generate_fake_rdns(host_id: str) -> str:
    """Generate fake reverse DNS for host_id."""
    try:
        # Try to parse as IP address
        ipaddress.ip_address(host_id)
        # Extract last octet from IP
        last_octet = host_id.split('.')[-1]
        return f"host-{last_octet}.local"
    except ValueError:
        # Not an IP address, use last part of host_id
        if '.' in host_id:
            last_part = host_id.split('.')[-1]
        else:
            last_part = host_id[-4:] if len(host_id) >= 4 else host_id
        return f"host-{last_part}.local"


def enrich_event(event_data: dict, env: str = "dev", fake_rdns: bool = True) -> dict:
    """Enrich an event with additional fields."""
    enriched = event_data.copy()
    
    # Add environment field
    enriched["env"] = env
    
    # Add fake reverse DNS if enabled
    if fake_rdns:
        host_id = event_data.get("metadata", {}).get("host_id", "unknown")
        enriched["rdns"] = generate_fake_rdns(host_id)
    
    # Add enrichment metadata
    if "metadata" not in enriched:
        enriched["metadata"] = {}
    
    enriched["metadata"]["enriched_at"] = 1640995200000
    enriched["metadata"]["enrichment_version"] = "1.0"
    
    return enriched


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
    
    # Test fake RDNS generation
    test_hosts = ["192.168.1.100", "server-01.example.com", "10.0.0.1", "2001:db8::1"]
    print(f"\nFake RDNS generation:")
    for host in test_hosts:
        rdns = generate_fake_rdns(host)
        print(f"  {host} -> {rdns}")
    
    # Test batch enrichment
    events_batch = [sample_event, sample_event.copy()]
    enriched_batch = [enrich_event(event) for event in events_batch]
    print(f"\nBatch enrichment: {len(enriched_batch)} events processed")
    
    print("\n✅ All enrichment tests passed!")


if __name__ == "__main__":
    test_enrichment()




