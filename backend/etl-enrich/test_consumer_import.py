#!/usr/bin/env python3
"""Test consumer import and basic functionality."""

def test_consumer_import():
    """Test that consumer can be imported."""
    try:
        import sys
        import os
        sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'app'))
        
        from consumer import EventConsumer, main
        print("✅ Consumer import successful")
        return True
    except Exception as e:
        print(f"❌ Consumer import failed: {e}")
        return False

def test_consumer_initialization():
    """Test consumer initialization."""
    try:
        import sys
        import os
        sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'app'))
        
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

def test_message_processing_components():
    """Test that message processing components can be imported."""
    try:
        import sys
        import os
        sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'app'))
        
        from consumer import EventConsumer
        from enrich import enrich_event
        import orjson
        import asyncio
        
        # Test basic functionality
        event_data = {
            "ts": "2024-01-01T12:00:00Z",
            "host_id": "host-001",
            "event_type": "connect",
            "args": {"dst_ip": "10.1.2.3", "dst_port": 80}
        }
        
        # Test JSON parsing
        json_str = orjson.dumps(event_data)
        parsed = orjson.loads(json_str)
        assert parsed == event_data
        
        # Test enrichment
        enriched = enrich_event(event_data, env="test", fake_rdns=True)
        assert "context" in enriched
        assert enriched["context"]["env"] == "test"
        assert enriched["context"]["rdns"] == "host-3.local"
        
        print("✅ Message processing components test passed")
        return True
    except Exception as e:
        print(f"❌ Message processing components test failed: {e}")
        return False

def main():
    """Run all tests."""
    print("Running consumer import and basic functionality tests...")
    print()
    
    tests = [
        test_consumer_import,
        test_consumer_initialization,
        test_message_processing_components
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
        print("🎉 All consumer tests passed!")
        return True
    else:
        print("❌ Some tests failed")
        return False

if __name__ == "__main__":
    success = main()
    exit(0 if success else 1)

