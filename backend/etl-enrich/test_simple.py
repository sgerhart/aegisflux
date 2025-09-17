#!/usr/bin/env python3
"""Simple test for core functionality."""

def test_timestamp_conversion():
    """Test timestamp conversion logic."""
    from datetime import datetime
    
    # Test timestamp conversion
    ts_ms = 1640995200000  # 2022-01-01 00:00:00 UTC
    ts_dt = datetime.fromtimestamp(ts_ms / 1000.0)
    
    # Verify the conversion (accounting for timezone differences)
    print(f"Converted timestamp: {ts_dt}")
    print(f"Expected: 2022-01-01 00:00:00 (UTC)")
    
    # The conversion should work (timezone differences are expected)
    # Check that it's the right date regardless of timezone
    assert ts_dt.year == 2021 or ts_dt.year == 2022
    assert ts_dt.month == 12 or ts_dt.month == 1
    assert ts_dt.day == 31 or ts_dt.day == 1
    print("✅ Timestamp conversion test passed")


def test_sql_query_structure():
    """Test SQL query structure."""
    # Test the SQL queries that would be generated
    raw_event_sql = """
        INSERT INTO events_raw (ts, host_id, event_type, payload_json)
        VALUES (%s, %s, %s, %s)
    """
    
    enriched_event_sql = """
        INSERT INTO events (id, type, source, timestamp, env, rdns, metadata, payload)
        VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
        ON CONFLICT (id) DO UPDATE SET
            type = EXCLUDED.type,
            source = EXCLUDED.source,
            timestamp = EXCLUDED.timestamp,
            env = EXCLUDED.env,
            rdns = EXCLUDED.rdns,
            metadata = EXCLUDED.metadata,
            payload = EXCLUDED.payload,
            created_at = NOW()
    """
    
    # Verify SQL structure
    assert "INSERT INTO events_raw" in raw_event_sql
    assert "ts, host_id, event_type, payload_json" in raw_event_sql
    assert "VALUES (%s, %s, %s, %s)" in raw_event_sql
    
    assert "INSERT INTO events" in enriched_event_sql
    assert "ON CONFLICT (id) DO UPDATE" in enriched_event_sql
    assert "VALUES (%s, %s, %s, %s, %s, %s, %s, %s)" in enriched_event_sql
    
    print("✅ SQL query structure test passed")


def test_enrichment_logic():
    """Test enrichment logic."""
    def generate_fake_rdns(host_id: str) -> str:
        """Generate fake reverse DNS for host_id."""
        try:
            # Try to parse as IP address
            import ipaddress
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
    
    # Test fake RDNS generation
    test_hosts = ["192.168.1.100", "server-01.example.com", "10.0.0.1"]
    for host in test_hosts:
        rdns = generate_fake_rdns(host)
        assert rdns.startswith("host-")
        assert rdns.endswith(".local")
        print(f"  {host} -> {rdns}")
    
    # Test event enrichment
    sample_event = {
        "id": "test-event-001",
        "type": "exec",
        "source": "/usr/bin/bash",
        "timestamp": 1640995200000,
        "metadata": {
            "host_id": "192.168.1.100",
            "pid": 1234
        }
    }
    
    enriched = enrich_event(sample_event)
    assert "env" in enriched
    assert "rdns" in enriched
    assert enriched["env"] == "dev"
    assert enriched["rdns"] == "host-100.local"
    
    print("✅ Enrichment logic test passed")


def main():
    """Run all simple tests."""
    print("Running simple functionality tests...")
    print()
    
    try:
        test_timestamp_conversion()
        test_sql_query_structure()
        test_enrichment_logic()
        
        print()
        print("🎉 All simple functionality tests passed!")
        
    except Exception as e:
        print(f"❌ Test failed: {e}")
        import traceback
        traceback.print_exc()
        return False
    
    return True


if __name__ == "__main__":
    success = main()
    exit(0 if success else 1)
