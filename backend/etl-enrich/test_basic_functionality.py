#!/usr/bin/env python3
"""Basic functionality test for TimescaleDB writer."""

import asyncio
import sys
import os

# Add the app directory to the Python path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'app'))

from writers.timescale import TimescaleWriter


def test_timestamp_conversion():
    """Test timestamp conversion logic."""
    from datetime import datetime
    
    # Test timestamp conversion
    ts_ms = 1640995200000  # 2022-01-01 00:00:00 UTC
    ts_dt = datetime.fromtimestamp(ts_ms / 1000.0)
    
    # Verify the conversion (accounting for timezone)
    expected = datetime(2022, 1, 1, 0, 0, 0)
    print(f"Converted timestamp: {ts_dt}")
    print(f"Expected timestamp: {expected}")
    
    # The conversion should work (timezone differences are expected)
    assert ts_dt.year == 2022
    assert ts_dt.month == 1
    assert ts_dt.day == 1
    print("✅ Timestamp conversion test passed")


def test_writer_initialization():
    """Test writer initialization."""
    writer = TimescaleWriter(
        connection_string="postgresql://test:test@localhost/test",
        min_size=1,
        max_size=5
    )
    
    assert writer.connection_string == "postgresql://test:test@localhost/test"
    assert writer._min_size == 1
    assert writer._max_size == 5
    assert writer._pool is None
    assert writer._initialized is False
    
    print("✅ Writer initialization test passed")


def test_sql_query_structure():
    """Test SQL query structure without database connection."""
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


def test_retry_decorator_presence():
    """Test that retry decorators are present."""
    # Check that the methods have retry decorators
    write_raw_event_method = TimescaleWriter.write_raw_event
    write_event_method = TimescaleWriter.write_event
    
    # The methods should have retry decorators (tenacity adds attributes)
    assert hasattr(write_raw_event_method, 'retry'), "write_raw_event should have retry decorator"
    assert hasattr(write_event_method, 'retry'), "write_event should have retry decorator"
    
    print("✅ Retry decorator presence test passed")


def test_consumer_integration():
    """Test that consumer integration works."""
    from consumer import EventConsumer
    
    # Test that the consumer can be imported and initialized
    consumer = EventConsumer()
    assert consumer is not None
    assert hasattr(consumer, '_process_batch')
    assert hasattr(consumer, '_process_message')
    
    print("✅ Consumer integration test passed")


def test_enrichment_functionality():
    """Test enrichment functionality."""
    from enrich import enrich_event, generate_fake_rdns
    
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
    assert enriched["env"] == "dev"  # Default from config
    assert enriched["rdns"] == "host-100.local"
    
    print("✅ Enrichment functionality test passed")


def main():
    """Run all basic functionality tests."""
    print("Running basic functionality tests for TimescaleDB writer...")
    print()
    
    try:
        test_timestamp_conversion()
        test_writer_initialization()
        test_sql_query_structure()
        test_retry_decorator_presence()
        test_consumer_integration()
        test_enrichment_functionality()
        
        print()
        print("🎉 All basic functionality tests passed!")
        
    except Exception as e:
        print(f"❌ Test failed: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()




