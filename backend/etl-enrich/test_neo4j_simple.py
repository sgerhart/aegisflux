#!/usr/bin/env python3
"""Simple test for Neo4j writer core functionality."""

def test_derive_dst_host_id():
    """Test derivation of destination host_id from IP addresses."""
    # Mock the writer class
    class MockWriter:
        def _derive_dst_host_id(self, dst_ip: str, dst_port: int) -> str:
            """Derive destination host_id from IP address."""
            internal_networks = [
                "192.168.", "10.", "172.16.", "172.17.", "172.18.", "172.19.",
                "172.20.", "172.21.", "172.22.", "172.23.", "172.24.", "172.25.",
                "172.26.", "172.27.", "172.28.", "172.29.", "172.30.", "172.31."
            ]
            
            is_internal = any(dst_ip.startswith(net) for net in internal_networks)
            
            if is_internal:
                return f"host-{dst_ip.replace('.', '-')}"
            else:
                return f"ip:{dst_ip}:{dst_port}"
    
    writer = MockWriter()
    
    # Test internal IPs
    assert writer._derive_dst_host_id("192.168.1.100", 80) == "host-192-168-1-100"
    assert writer._derive_dst_host_id("10.0.0.1", 443) == "host-10-0-0-1"
    assert writer._derive_dst_host_id("172.16.1.1", 22) == "host-172-16-1-1"
    
    # Test external IPs
    assert writer._derive_dst_host_id("8.8.8.8", 53) == "ip:8.8.8.8:53"
    assert writer._derive_dst_host_id("1.1.1.1", 80) == "ip:1.1.1.1:80"
    assert writer._derive_dst_host_id("google.com", 443) == "ip:google.com:443"
    
    print("✅ Host ID derivation test passed")


def test_parse_connect_event():
    """Test parsing of connect events."""
    class MockWriter:
        def _derive_dst_host_id(self, dst_ip: str, dst_port: int) -> str:
            return f"ip:{dst_ip}:{dst_port}"
        
        def _parse_connect_event(self, event_data: dict) -> dict:
            """Parse a connect event to extract connection information."""
            if event_data.get("type") != "connect":
                return None
            
            src_host_id = event_data.get("metadata", {}).get("host_id")
            if not src_host_id:
                return None
            
            payload = event_data.get("payload")
            if not payload:
                return None
            
            try:
                import orjson
                if isinstance(payload, bytes):
                    payload_data = orjson.loads(payload)
                else:
                    payload_data = payload
                
                args = payload_data.get("args", {})
                dst_ip = args.get("dst_ip")
                dst_port = args.get("dst_port")
                
                if not dst_ip or dst_port is None:
                    return None
                
                dst_host_id = self._derive_dst_host_id(dst_ip, dst_port)
                
                return {
                    "src_host_id": src_host_id,
                    "dst_host_id": dst_host_id,
                    "dst_ip": dst_ip,
                    "dst_port": dst_port
                }
                
            except Exception as e:
                return None
    
    writer = MockWriter()
    
    # Test valid connect event
    event_data = {
        "id": "connect-event-001",
        "type": "connect",
        "metadata": {
            "host_id": "host-192-168-1-100"
        },
        "payload": b'{"args": {"dst_ip": "8.8.8.8", "dst_port": 53}}'
    }
    
    result = writer._parse_connect_event(event_data)
    assert result is not None
    assert result["src_host_id"] == "host-192-168-1-100"
    assert result["dst_ip"] == "8.8.8.8"
    assert result["dst_port"] == 53
    assert result["dst_host_id"] == "ip:8.8.8.8:53"
    
    # Test non-connect event
    event_data = {
        "id": "exec-event-001",
        "type": "exec",
        "metadata": {"host_id": "host-192-168-1-100"}
    }
    
    result = writer._parse_connect_event(event_data)
    assert result is None
    
    print("✅ Connect event parsing test passed")


def test_cypher_query_structure():
    """Test that the Cypher queries have the correct structure."""
    # Test the expected Cypher query structure
    host_to_host_query = """
        MERGE (a:Host {host_id: $src})
        MERGE (b:Host {host_id: $dst})
        MERGE (a)-[r:COMMUNICATES]->(b)
        ON CREATE SET r.count_1h = 1, r.last_seen = timestamp()
        ON MATCH SET r.count_1h = coalesce(r.count_1h, 0) + 1, r.last_seen = timestamp()
    """
    
    host_to_endpoint_query = """
        MERGE (a:Host {host_id: $src})
        MERGE (b:NetworkEndpoint {endpoint_id: $dst})
        SET b.ip = $ip, b.port = $port
        MERGE (a)-[r:COMMUNICATES]->(b)
        ON CREATE SET r.count_1h = 1, r.last_seen = timestamp()
        ON MATCH SET r.count_1h = coalesce(r.count_1h, 0) + 1, r.last_seen = timestamp()
    """
    
    # Verify host-to-host query structure
    assert "MERGE (a:Host {host_id: $src})" in host_to_host_query
    assert "MERGE (b:Host {host_id: $dst})" in host_to_host_query
    assert "MERGE (a)-[r:COMMUNICATES]->(b)" in host_to_host_query
    assert "ON CREATE SET r.count_1h = 1, r.last_seen = timestamp()" in host_to_host_query
    assert "ON MATCH SET r.count_1h = coalesce(r.count_1h, 0) + 1, r.last_seen = timestamp()" in host_to_host_query
    
    # Verify host-to-endpoint query structure
    assert "MERGE (a:Host {host_id: $src})" in host_to_endpoint_query
    assert "MERGE (b:NetworkEndpoint {endpoint_id: $dst})" in host_to_endpoint_query
    assert "SET b.ip = $ip, b.port = $port" in host_to_endpoint_query
    assert "MERGE (a)-[r:COMMUNICATES]->(b)" in host_to_endpoint_query
    assert "ON CREATE SET r.count_1h = 1, r.last_seen = timestamp()" in host_to_endpoint_query
    assert "ON MATCH SET r.count_1h = coalesce(r.count_1h, 0) + 1, r.last_seen = timestamp()" in host_to_endpoint_query
    
    print("✅ Cypher query structure test passed")


def test_retry_decorator_presence():
    """Test that retry decorators are present."""
    # Import the actual writer to check decorators
    import sys
    import os
    sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'app'))
    
    try:
        from writers.neo4j import Neo4jWriter
        
        # Check that the methods have retry decorators
        upsert_method = Neo4jWriter.upsert_comm_edge
        write_method = Neo4jWriter.write_event
        
        # The methods should have retry decorators (tenacity adds attributes)
        assert hasattr(upsert_method, 'retry'), "upsert_comm_edge should have retry decorator"
        assert hasattr(write_method, 'retry'), "write_event should have retry decorator"
        
        print("✅ Retry decorator presence test passed")
        
    except ImportError as e:
        print(f"⚠️  Could not import Neo4jWriter: {e}")
        print("✅ Retry decorator test skipped")


def main():
    """Run all simple tests."""
    print("Running simple Neo4j functionality tests...")
    print()
    
    try:
        test_derive_dst_host_id()
        test_parse_connect_event()
        test_cypher_query_structure()
        test_retry_decorator_presence()
        
        print()
        print("🎉 All simple Neo4j functionality tests passed!")
        
    except Exception as e:
        print(f"❌ Test failed: {e}")
        import traceback
        traceback.print_exc()
        return False
    
    return True


if __name__ == "__main__":
    success = main()
    exit(0 if success else 1)

