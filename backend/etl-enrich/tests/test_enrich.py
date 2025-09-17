"""Tests for event enrichment functionality."""

import pytest
from app.enrich import enrich_event, enrich_events_batch, validate_enriched_event, is_ipv4_address, extract_last_octet


class TestEnrichment:
    """Test cases for event enrichment."""
    
    def test_adds_env(self):
        """Test that enrich_event adds env field to context."""
        event = {
            "id": "test-event-001",
            "type": "exec",
            "args": {"dst_ip": "10.1.2.3"}
        }
        
        result = enrich_event(event, env="dev", fake_rdns=True)
        
        # Should have context field
        assert "context" in result
        assert result["context"]["env"] == "dev"
        
        # Original event should not be modified
        assert "context" not in event
    
    def test_rdns_added_when_dst_ip_is_ipv4(self):
        """Test that rdns is added when dst_ip is IPv4."""
        event = {
            "id": "test-event-001",
            "type": "connect",
            "args": {"dst_ip": "10.1.2.3", "dst_port": 80}
        }
        
        result = enrich_event(event, env="prod", fake_rdns=True)
        
        # Should have context with rdns
        assert "context" in result
        assert result["context"]["env"] == "prod"
        assert result["context"]["rdns"] == "host-3.local"
    
    def test_rdns_omitted_when_no_dst_ip(self):
        """Test that rdns is omitted when no dst_ip."""
        event = {
            "id": "test-event-001",
            "type": "exec",
            "args": {"pid": 1234}
        }
        
        result = enrich_event(event, env="dev", fake_rdns=True)
        
        # Should have context but no rdns
        assert "context" in result
        assert result["context"]["env"] == "dev"
        assert result["context"]["rdns"] is None
    
    def test_rdns_omitted_when_dst_ip_not_ipv4(self):
        """Test that rdns is omitted when dst_ip is not IPv4."""
        event = {
            "id": "test-event-001",
            "type": "connect",
            "args": {"dst_ip": "2001:db8::1", "dst_port": 80}
        }
        
        result = enrich_event(event, env="dev", fake_rdns=True)
        
        # Should have context but no rdns
        assert "context" in result
        assert result["context"]["env"] == "dev"
        assert result["context"]["rdns"] is None
    
    def test_rdns_omitted_when_fake_rdns_false(self):
        """Test that rdns is omitted when fake_rdns is False."""
        event = {
            "id": "test-event-001",
            "type": "connect",
            "args": {"dst_ip": "10.1.2.3", "dst_port": 80}
        }
        
        result = enrich_event(event, env="dev", fake_rdns=False)
        
        # Should have context but no rdns
        assert "context" in result
        assert result["context"]["env"] == "dev"
        assert result["context"]["rdns"] is None
    
    def test_different_ipv4_addresses(self):
        """Test rdns generation for different IPv4 addresses."""
        test_cases = [
            ("192.168.1.100", "host-100.local"),
            ("10.0.0.1", "host-1.local"),
            ("172.16.254.255", "host-255.local"),
            ("127.0.0.1", "host-1.local"),
            ("0.0.0.0", "host-0.local")
        ]
        
        for dst_ip, expected_rdns in test_cases:
            event = {
                "id": f"test-event-{dst_ip}",
                "type": "connect",
                "args": {"dst_ip": dst_ip, "dst_port": 80}
            }
            
            result = enrich_event(event, env="dev", fake_rdns=True)
            
            assert result["context"]["rdns"] == expected_rdns, f"Failed for {dst_ip}"
    
    def test_invalid_ip_addresses(self):
        """Test that invalid IP addresses don't generate rdns."""
        invalid_ips = [
            "not-an-ip",
            "256.1.1.1",
            "1.1.1.1.1",
            "1.1.1",
            "",
            None,
            "2001:db8::1",  # IPv6
            "localhost",
            "example.com"
        ]
        
        for invalid_ip in invalid_ips:
            event = {
                "id": f"test-event-{invalid_ip}",
                "type": "connect",
                "args": {"dst_ip": invalid_ip, "dst_port": 80}
            }
            
            result = enrich_event(event, env="dev", fake_rdns=True)
            
            assert result["context"]["rdns"] is None, f"Should be None for {invalid_ip}"
    
    def test_batch_enrichment(self):
        """Test batch enrichment of events."""
        events = [
            {
                "id": "event-1",
                "type": "connect",
                "args": {"dst_ip": "10.1.2.3", "dst_port": 80}
            },
            {
                "id": "event-2",
                "type": "exec",
                "args": {"pid": 1234}
            },
            {
                "id": "event-3",
                "type": "connect",
                "args": {"dst_ip": "192.168.1.100", "dst_port": 443}
            }
        ]
        
        results = enrich_events_batch(events, env="test", fake_rdns=True)
        
        assert len(results) == 3
        
        # Check first event (has IPv4 dst_ip)
        assert results[0]["context"]["env"] == "test"
        assert results[0]["context"]["rdns"] == "host-3.local"
        
        # Check second event (no dst_ip)
        assert results[1]["context"]["env"] == "test"
        assert results[1]["context"]["rdns"] is None
        
        # Check third event (has IPv4 dst_ip)
        assert results[2]["context"]["env"] == "test"
        assert results[2]["context"]["rdns"] == "host-100.local"
    
    def test_validate_enriched_event(self):
        """Test validation of enriched events."""
        # Valid enriched event
        valid_event = {
            "id": "test-event",
            "context": {
                "env": "dev",
                "rdns": "host-3.local"
            }
        }
        assert validate_enriched_event(valid_event) is True
        
        # Missing context
        invalid_event1 = {
            "id": "test-event",
            "env": "dev"
        }
        assert validate_enriched_event(invalid_event1) is False
        
        # Missing env in context
        invalid_event2 = {
            "id": "test-event",
            "context": {
                "rdns": "host-3.local"
            }
        }
        assert validate_enriched_event(invalid_event2) is False


class TestIPv4Detection:
    """Test cases for IPv4 address detection."""
    
    def test_is_ipv4_address_valid(self):
        """Test IPv4 detection with valid addresses."""
        valid_ips = [
            "192.168.1.1",
            "10.0.0.1",
            "172.16.254.255",
            "127.0.0.1",
            "0.0.0.0",
            "255.255.255.255"
        ]
        
        for ip in valid_ips:
            assert is_ipv4_address(ip), f"Should be valid IPv4: {ip}"
    
    def test_is_ipv4_address_invalid(self):
        """Test IPv4 detection with invalid addresses."""
        invalid_ips = [
            "not-an-ip",
            "256.1.1.1",
            "1.1.1.1.1",
            "1.1.1",
            "",
            None,
            "2001:db8::1",  # IPv6
            "localhost",
            "example.com"
        ]
        
        for ip in invalid_ips:
            assert not is_ipv4_address(ip), f"Should be invalid IPv4: {ip}"
    
    def test_extract_last_octet(self):
        """Test extraction of last octet from IPv4 addresses."""
        test_cases = [
            ("192.168.1.100", "100"),
            ("10.0.0.1", "1"),
            ("172.16.254.255", "255"),
            ("127.0.0.1", "1"),
            ("0.0.0.0", "0")
        ]
        
        for ip, expected_octet in test_cases:
            result = extract_last_octet(ip)
            assert result == expected_octet, f"Failed for {ip}"
    
    def test_extract_last_octet_invalid(self):
        """Test extraction of last octet from invalid addresses."""
        invalid_ips = [
            "not-an-ip",
            "256.1.1.1",
            "2001:db8::1",
            "localhost",
            "",
            None
        ]
        
        for ip in invalid_ips:
            result = extract_last_octet(ip)
            assert result is None, f"Should be None for {ip}"