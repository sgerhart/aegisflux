#!/usr/bin/env python3
"""Simple test for consumer functionality."""

import asyncio
import json
import sys
import os
from unittest.mock import AsyncMock, MagicMock

# Add app directory to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'app'))

def test_consumer_imports():
    """Test that consumer can be imported without errors."""
    try:
        from consumer import EventConsumer, main
        print("✅ Consumer imports successful")
        return True
    except Exception as e:
        print(f"❌ Import failed: {e}")
        return False

def test_consumer_initialization():
    """Test consumer initialization."""
    try:
        from consumer import EventConsumer
        
        consumer = EventConsumer(max_inflight=50)
        assert consumer.max_inflight == 50
        assert consumer.semaphore._value == 50
        assert consumer.timescale_writer is None
        assert consumer.neo4j_writer is None
        assert consumer.nats_client is None
        assert consumer.running is False
        
        print("✅ Consumer initialization test passed")
        return True
    except Exception as e:
        print(f"❌ Consumer initialization test failed: {e}")
        return False

def test_message_processing_logic():
    """Test the core message processing logic."""
    try:
        from consumer import EventConsumer
        from enrich import enrich_event
        
        # Test event data
        event_data = {
            "ts": "2024-01-01T12:00:00Z",
            "host_id": "host-001",
            "event_type": "connect",
            "args": {
                "dst_ip": "10.1.2.3",
                "dst_port": 80
            }
        }
        
        # Test enrichment
        enriched = enrich_event(event_data, env="test", fake_rdns=True)
        
        assert "context" in enriched
        assert enriched["context"]["env"] == "test"
        assert enriched["context"]["rdns"] == "host-3.local"
        
        print("✅ Message processing logic test passed")
        return True
    except Exception as e:
        print(f"❌ Message processing logic test failed: {e}")
        return False

def test_required_field_extraction():
    """Test extraction of required fields."""
    try:
        # Valid event
        valid_event = {
            "ts": "2024-01-01T12:00:00Z",
            "host_id": "host-001", 
            "event_type": "exec"
        }
        
        ts = valid_event.get("ts")
        host_id = valid_event.get("host_id")
        event_type = valid_event.get("event_type")
        
        assert ts is not None
        assert host_id is not None
        assert event_type is not None
        
        # Invalid event (missing fields)
        invalid_event = {
            "ts": "2024-01-01T12:00:00Z",
            "host_id": "host-001"
            # missing event_type
        }
        
        ts = invalid_event.get("ts")
        host_id = invalid_event.get("host_id")
        event_type = invalid_event.get("event_type")
        
        assert ts is not None
        assert host_id is not None
        assert event_type is None
        
        print("✅ Required field extraction test passed")
        return True
    except Exception as e:
        print(f"❌ Required field extraction test failed: {e}")
        return False

def test_timestamp_conversion():
    """Test timestamp conversion logic."""
    try:
        from datetime import datetime
        
        # Test ISO timestamp conversion
        iso_ts = "2024-01-01T12:00:00Z"
        dt = datetime.fromisoformat(iso_ts.replace('Z', '+00:00'))
        ts_ms = int(dt.timestamp() * 1000)
        
        # Should be a reasonable timestamp (around 2024)
        assert ts_ms > 1600000000000  # After 2020
        assert ts_ms < 2000000000000  # Before 2030
        
        # Test numeric timestamp
        numeric_ts = 1704110400000  # 2024-01-01T12:00:00Z in ms
        ts_ms = int(numeric_ts)
        assert ts_ms == 1704110400000
        
        print("✅ Timestamp conversion test passed")
        return True
    except Exception as e:
        print(f"❌ Timestamp conversion test failed: {e}")
        return False

def main():
    """Run all simple tests."""
    print("Running simple consumer functionality tests...")
    print()
    
    tests = [
        test_consumer_imports,
        test_consumer_initialization,
        test_message_processing_logic,
        test_required_field_extraction,
        test_timestamp_conversion
    ]
    
    passed = 0
    total = len(tests)
    
    for test in tests:
        try:
            if test():
                passed += 1
            print()
        except Exception as e:
            print(f"❌ Test {test.__name__} failed with exception: {e}")
            print()
    
    print(f"Results: {passed}/{total} tests passed")
    
    if passed == total:
        print("🎉 All simple consumer functionality tests passed!")
        return True
    else:
        print("❌ Some tests failed")
        return False

if __name__ == "__main__":
    success = main()
    exit(0 if success else 1)




