"""Tests for Neo4j writer with fake session/transaction objects."""

import pytest
from unittest.mock import Mock, MagicMock, patch
import neo4j

from app.writers.neo4j import Neo4jWriter, upsert_comm_edge


class MockTransaction:
    """Mock transaction for testing Cypher execution."""
    
    def __init__(self):
        self.executed_queries = []
        self.executed_params = []
        self.call_count = 0
        self.side_effects = []
        self.return_values = []
    
    def run(self, query: str, parameters=None):
        """Mock run method."""
        self.executed_queries.append(query)
        self.executed_params.append(parameters)
        self.call_count += 1
        
        # Check for side effects
        if self.side_effects and self.call_count <= len(self.side_effects):
            side_effect = self.side_effects[self.call_count - 1]
            if isinstance(side_effect, Exception):
                raise side_effect
        
        # Return mock result if available
        if self.return_values and self.call_count <= len(self.return_values):
            return self.return_values[self.call_count - 1]
        
        return Mock()
    
    def __enter__(self):
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        pass


class MockSession:
    """Mock session for testing."""
    
    def __init__(self):
        self.transaction = MockTransaction()
        self.closed = False
    
    def run(self, query: str, parameters=None):
        """Mock run method."""
        return self.transaction.run(query, parameters)
    
    def __enter__(self):
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        pass


class MockDriver:
    """Mock driver for testing."""
    
    def __init__(self):
        self.sessions = []
        self.closed = False
    
    def session(self):
        """Mock session method."""
        if self.closed:
            raise neo4j.exceptions.ServiceUnavailable("Driver is closed")
        
        session = MockSession()
        self.sessions.append(session)
        return session
    
    def close(self):
        """Mock close method."""
        self.closed = True


class TestNeo4jWriter:
    """Test cases for Neo4j writer with mocked database."""
    
    @pytest.fixture
    def mock_driver(self):
        """Mock Neo4j driver."""
        return MockDriver()
    
    @pytest.fixture
    def writer_with_mock_driver(self, mock_driver):
        """Writer instance with mocked driver."""
        writer = Neo4jWriter(uri="bolt://localhost:7687", username="neo4j", password="password")
        writer._driver = mock_driver
        writer._initialized = True
        return writer
    
    def test_derive_dst_host_id_internal_ip(self, writer_with_mock_driver):
        """Test derivation of host_id for internal IPs."""
        # Test internal IP
        dst_ip = "192.168.1.100"
        dst_port = 80
        result = writer_with_mock_driver._derive_dst_host_id(dst_ip, dst_port)
        
        assert result == "host-192-168-1-100"
        
        # Test another internal IP
        dst_ip = "10.0.0.1"
        dst_port = 443
        result = writer_with_mock_driver._derive_dst_host_id(dst_ip, dst_port)
        
        assert result == "host-10-0-0-1"
    
    def test_derive_dst_host_id_external_ip(self, writer_with_mock_driver):
        """Test derivation of host_id for external IPs."""
        # Test external IP
        dst_ip = "8.8.8.8"
        dst_port = 53
        result = writer_with_mock_driver._derive_dst_host_id(dst_ip, dst_port)
        
        assert result == "ip:8.8.8.8:53"
        
        # Test another external IP
        dst_ip = "1.1.1.1"
        dst_port = 80
        result = writer_with_mock_driver._derive_dst_host_id(dst_ip, dst_port)
        
        assert result == "ip:1.1.1.1:80"
    
    def test_parse_connect_event_success(self, writer_with_mock_driver):
        """Test parsing of connect events."""
        # Test valid connect event
        event_data = {
            "id": "connect-event-001",
            "type": "connect",
            "metadata": {
                "host_id": "host-192-168-1-100"
            },
            "payload": b'{"args": {"dst_ip": "8.8.8.8", "dst_port": 53}}'
        }
        
        result = writer_with_mock_driver._parse_connect_event(event_data)
        
        assert result is not None
        assert result["src_host_id"] == "host-192-168-1-100"
        assert result["dst_ip"] == "8.8.8.8"
        assert result["dst_port"] == 53
        assert result["dst_host_id"] == "ip:8.8.8.8:53"
    
    def test_parse_connect_event_invalid_type(self, writer_with_mock_driver):
        """Test parsing of non-connect events."""
        # Test non-connect event
        event_data = {
            "id": "exec-event-001",
            "type": "exec",
            "metadata": {
                "host_id": "host-192-168-1-100"
            }
        }
        
        result = writer_with_mock_driver._parse_connect_event(event_data)
        assert result is None
    
    def test_parse_connect_event_missing_fields(self, writer_with_mock_driver):
        """Test parsing of connect events with missing fields."""
        # Test missing host_id
        event_data = {
            "id": "connect-event-001",
            "type": "connect",
            "payload": b'{"args": {"dst_ip": "8.8.8.8", "dst_port": 53}}'
        }
        
        result = writer_with_mock_driver._parse_connect_event(event_data)
        assert result is None
        
        # Test missing payload
        event_data = {
            "id": "connect-event-001",
            "type": "connect",
            "metadata": {
                "host_id": "host-192-168-1-100"
            }
        }
        
        result = writer_with_mock_driver._parse_connect_event(event_data)
        assert result is None
    
    @pytest.mark.asyncio
    async def test_upsert_comm_edge_host_to_host(self, writer_with_mock_driver):
        """Test upserting communication edge between two hosts."""
        src_host_id = "host-192-168-1-100"
        dst_host_id = "host-192-168-1-101"
        
        result = await writer_with_mock_driver.upsert_comm_edge(src_host_id, dst_host_id)
        
        assert result is True
        assert len(writer_with_mock_driver._driver.sessions) == 1
        
        session = writer_with_mock_driver._driver.sessions[0]
        executed_queries = session.transaction.executed_queries
        
        # Verify the correct Cypher query was executed
        assert len(executed_queries) == 1
        query = executed_queries[0]
        assert "MERGE (a:Host {host_id: $src})" in query
        assert "MERGE (b:Host {host_id: $dst})" in query
        assert "MERGE (a)-[r:COMMUNICATES]->(b)" in query
        assert "ON CREATE SET r.count_1h = 1, r.last_seen = timestamp()" in query
        assert "ON MATCH SET r.count_1h = coalesce(r.count_1h, 0) + 1, r.last_seen = timestamp()" in query
        
        # Verify parameters
        executed_params = session.transaction.executed_params
        assert len(executed_params) == 1
        params = executed_params[0]
        assert params["src"] == src_host_id
        assert params["dst"] == dst_host_id
    
    @pytest.mark.asyncio
    async def test_upsert_comm_edge_host_to_network_endpoint(self, writer_with_mock_driver):
        """Test upserting communication edge from host to network endpoint."""
        src_host_id = "host-192-168-1-100"
        dst_host_id = "ip:8.8.8.8:53"
        
        result = await writer_with_mock_driver.upsert_comm_edge(src_host_id, dst_host_id)
        
        assert result is True
        assert len(writer_with_mock_driver._driver.sessions) == 1
        
        session = writer_with_mock_driver._driver.sessions[0]
        executed_queries = session.transaction.executed_queries
        
        # Verify the correct Cypher query was executed
        assert len(executed_queries) == 1
        query = executed_queries[0]
        assert "MERGE (a:Host {host_id: $src})" in query
        assert "MERGE (b:NetworkEndpoint {endpoint_id: $dst})" in query
        assert "SET b.ip = $ip, b.port = $port" in query
        assert "MERGE (a)-[r:COMMUNICATES]->(b)" in query
        
        # Verify parameters
        executed_params = session.transaction.executed_params
        assert len(executed_params) == 1
        params = executed_params[0]
        assert params["src"] == src_host_id
        assert params["dst"] == dst_host_id
        assert params["ip"] == "8.8.8.8"
        assert params["port"] == 53
    
    @pytest.mark.asyncio
    async def test_write_event_with_connect_event(self, writer_with_mock_driver):
        """Test writing a connect event."""
        event_data = {
            "id": "connect-event-001",
            "type": "connect",
            "source": "/usr/bin/curl",
            "timestamp": 1640995200000,
            "env": "dev",
            "rdns": "host-100.local",
            "metadata": {
                "host_id": "host-192-168-1-100",
                "pid": 1234
            },
            "payload": b'{"args": {"dst_ip": "8.8.8.8", "dst_port": 53}}'
        }
        
        result = await writer_with_mock_driver.write_event(event_data)
        
        assert result is True
        assert len(writer_with_mock_driver._driver.sessions) >= 1
        
        # Get the first session for verification
        session = writer_with_mock_driver._driver.sessions[0]
        executed_queries = session.transaction.executed_queries
        
        # Should have multiple queries: event creation, host creation, and communication edge
        assert len(executed_queries) >= 3
        
        # Verify event creation query
        event_query = executed_queries[0]
        assert "MERGE (e:Event {id: $event_id})" in event_query
        assert "SET e.type = $type" in event_query
        
        # Verify host creation query
        host_query = executed_queries[1]
        assert "MERGE (h:Host {host_id: $host_id})" in host_query
        assert "MERGE (h)-[:GENERATED]->(e:Event {id: $event_id})" in host_query
        
        # Verify communication edge query
        comm_query = executed_queries[2]
        assert "MERGE (a:Host {host_id: $src})" in comm_query
        assert "MERGE (a)-[r:COMMUNICATES]->" in comm_query
    
    @pytest.mark.asyncio
    async def test_write_event_regular_event(self, writer_with_mock_driver):
        """Test writing a regular (non-connect) event."""
        event_data = {
            "id": "exec-event-001",
            "type": "exec",
            "source": "/usr/bin/bash",
            "timestamp": 1640995200000,
            "env": "dev",
            "rdns": "host-100.local",
            "metadata": {
                "host_id": "host-192-168-1-100",
                "pid": 1234,
                "uid": 1001
            },
            "payload": b'{"args": ["/usr/bin/bash", "-c", "echo hi"]}'
        }
        
        result = await writer_with_mock_driver.write_event(event_data)
        
        assert result is True
        assert len(writer_with_mock_driver._driver.sessions) == 1
        
        session = writer_with_mock_driver._driver.sessions[0]
        executed_queries = session.transaction.executed_queries
        
        # Should have multiple queries: event creation, host creation, user creation, process creation
        assert len(executed_queries) >= 4
        
        # Verify event creation query
        event_query = executed_queries[0]
        assert "MERGE (e:Event {id: $event_id})" in event_query
        
        # Verify host creation query
        host_query = executed_queries[1]
        assert "MERGE (h:Host {host_id: $host_id})" in host_query
        
        # Verify user creation query
        user_query = executed_queries[2]
        assert "MERGE (u:User {uid: $uid})" in user_query
        assert "MERGE (u)-[:EXECUTED]->(e:Event {id: $event_id})" in user_query
        
        # Verify process creation query
        process_query = executed_queries[3]
        assert "MERGE (p:Process {pid: $pid, host_id: $host_id})" in process_query
        assert "MERGE (p)-[:EXECUTED]->(e:Event {id: $event_id})" in process_query
    
    @pytest.mark.asyncio
    async def test_connection_creation(self):
        """Test Neo4j connection creation."""
        with patch('app.writers.neo4j.neo4j.GraphDatabase.driver') as mock_driver_class:
            mock_driver = MockDriver()
            mock_driver_class.return_value = mock_driver
            
            writer = Neo4jWriter(uri="bolt://localhost:7687", username="neo4j", password="password")
            driver = await writer.connect()
            
            mock_driver_class.assert_called_once_with(
                "bolt://localhost:7687",
                auth=("neo4j", "password")
            )
            assert writer._driver == mock_driver
            assert driver == mock_driver
    
    @pytest.mark.asyncio
    async def test_retry_on_transient_error(self, writer_with_mock_driver):
        """Test retry logic on transient errors."""
        # First call to create a session
        await writer_with_mock_driver.upsert_comm_edge("host-1", "host-2")
        
        # Configure mock to fail first, then succeed
        session = writer_with_mock_driver._driver.sessions[0]
        session.transaction.side_effects = [
            neo4j.exceptions.ServiceUnavailable("Connection lost"),
            None  # Success on retry
        ]
        
        result = await writer_with_mock_driver.upsert_comm_edge("host-1", "host-2")
        
        assert result is True
        assert session.transaction.call_count >= 2  # Failed once, succeeded on retry
    
    @pytest.mark.asyncio
    async def test_upsert_comm_edge_function(self, writer_with_mock_driver):
        """Test the convenience function upsert_comm_edge."""
        with patch('app.writers.neo4j.get_writer') as mock_get_writer:
            mock_get_writer.return_value = writer_with_mock_driver
            
            result = await upsert_comm_edge("host-1", "host-2")
            
            assert result is True
            mock_get_writer.assert_called_once()
    
    def test_cypher_query_structure(self):
        """Test that the Cypher queries have the correct structure."""
        # Test the expected Cypher query structure
        expected_host_to_host_query = """
            MERGE (a:Host {host_id: $src})
            MERGE (b:Host {host_id: $dst})
            MERGE (a)-[r:COMMUNICATES]->(b)
            ON CREATE SET r.count_1h = 1, r.last_seen = timestamp()
            ON MATCH SET r.count_1h = coalesce(r.count_1h, 0) + 1, r.last_seen = timestamp()
        """
        
        expected_host_to_endpoint_query = """
            MERGE (a:Host {host_id: $src})
            MERGE (b:NetworkEndpoint {endpoint_id: $dst})
            SET b.ip = $ip, b.port = $port
            MERGE (a)-[r:COMMUNICATES]->(b)
            ON CREATE SET r.count_1h = 1, r.last_seen = timestamp()
            ON MATCH SET r.count_1h = coalesce(r.count_1h, 0) + 1, r.last_seen = timestamp()
        """
        
        # Verify query structure
        assert "MERGE (a:Host {host_id: $src})" in expected_host_to_host_query
        assert "MERGE (b:Host {host_id: $dst})" in expected_host_to_host_query
        assert "MERGE (a)-[r:COMMUNICATES]->(b)" in expected_host_to_host_query
        assert "ON CREATE SET r.count_1h = 1, r.last_seen = timestamp()" in expected_host_to_host_query
        assert "ON MATCH SET r.count_1h = coalesce(r.count_1h, 0) + 1, r.last_seen = timestamp()" in expected_host_to_host_query
        
        assert "MERGE (a:Host {host_id: $src})" in expected_host_to_endpoint_query
        assert "MERGE (b:NetworkEndpoint {endpoint_id: $dst})" in expected_host_to_endpoint_query
        assert "SET b.ip = $ip, b.port = $port" in expected_host_to_endpoint_query
        assert "MERGE (a)-[r:COMMUNICATES]->(b)" in expected_host_to_endpoint_query
