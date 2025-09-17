#!/usr/bin/env python3
"""Simple test for enrichment functionality."""

def test_enrichment_core_functionality():
    """Test the core enrichment functionality."""
    import sys
    import os
    sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'app'))
    
    from enrich import enrich_event, is_ipv4_address, extract_last_octet
    
    # Test IPv4 detection
    assert is_ipv4_address("10.1.2.3") == True
    assert is_ipv4_address("192.168.1.100") == True
    assert is_ipv4_address("2001:db8::1") == False
    assert is_ipv4_address("not-an-ip") == False
    print("✅ IPv4 detection test passed")
    
    # Test last octet extraction
    assert extract_last_octet("10.1.2.3") == "3"
    assert extract_last_octet("192.168.1.100") == "100"
    assert extract_last_octet("2001:db8::1") == None
    print("✅ Last octet extraction test passed")
    
    # Test enrichment with IPv4 dst_ip
    event = {
        "id": "test-event-001",
        "type": "connect",
        "args": {"dst_ip": "10.1.2.3", "dst_port": 80}
    }
    
    result = enrich_event(event, env="dev", fake_rdns=True)
    
    assert "context" in result
    assert result["context"]["env"] == "dev"
    assert result["context"]["rdns"] == "host-3.local"
    print("✅ Enrichment with IPv4 test passed")
    
    # Test enrichment without dst_ip
    event2 = {
        "id": "test-event-002",
        "type": "exec",
        "args": {"pid": 1234}
    }
    
    result2 = enrich_event(event2, env="prod", fake_rdns=True)
    
    assert "context" in result2
    assert result2["context"]["env"] == "prod"
    assert result2["context"]["rdns"] is None
    print("✅ Enrichment without dst_ip test passed")
    
    # Test enrichment with IPv6 dst_ip
    event3 = {
        "id": "test-event-003",
        "type": "connect",
        "args": {"dst_ip": "2001:db8::1", "dst_port": 80}
    }
    
    result3 = enrich_event(event3, env="test", fake_rdns=True)
    
    assert "context" in result3
    assert result3["context"]["env"] == "test"
    assert result3["context"]["rdns"] is None
    print("✅ Enrichment with IPv6 test passed")
    
    # Test that original event is not modified
    assert "context" not in event
    assert "context" not in event2
    assert "context" not in event3
    print("✅ Original event preservation test passed")


def main():
    """Run all simple tests."""
    print("Running simple enrichment functionality tests...")
    print()
    
    try:
        test_enrichment_core_functionality()
        
        print()
        print("🎉 All simple enrichment functionality tests passed!")
        
    except Exception as e:
        print(f"❌ Test failed: {e}")
        import traceback
        traceback.print_exc()
        return False
    
    return True


if __name__ == "__main__":
    success = main()
    exit(0 if success else 1)

