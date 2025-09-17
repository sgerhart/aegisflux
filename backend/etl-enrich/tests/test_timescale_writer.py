"""Tests for TimescaleDB writer with DB stub."""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch
from datetime import datetime
import psycopg

from app.writers.timescale import TimescaleWriter


class MockCursor:
    """Mock cursor for testing SQL execution."""
    
    def __init__(self):
        self.executed_queries = []
        self.executed_params = []
        self.return_values = []
        self.side_effects = []
        self.call_count = 0
    
    async def execute(self, query: str, params=None):
        """Mock execute method."""
        self.executed_queries.append(query)
        self.executed_params.append(params)
        self.call_count += 1
        
        # Check for side effects
        if self.side_effects and self.call_count <= len(self.side_effects):
            side_effect = self.side_effects[self.call_count - 1]
            if isinstance(side_effect, Exception):
                raise side_effect
        
        # Return mock result if available
        if self.return_values and self.call_count <= len(self.return_values):
            return self.return_values[self.call_count - 1]
    
    async def __aenter__(self):
        return self
    
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        pass


class MockConnection:
    """Mock connection for testing."""
    
    def __init__(self):
        self._cursor = MockCursor()
        self.committed = False
        self.closed = False
    
    def cursor(self):
        """Mock cursor method that returns an async context manager."""
        return self._cursor
    
    async def commit(self):
        """Mock commit method."""
        self.committed = True
    
    async def close(self):
        """Mock close method."""
        self.closed = True
    
    async def __aenter__(self):
        return self
    
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        pass


class MockConnectionPool:
    """Mock connection pool for testing."""
    
    def __init__(self):
        self.connections = []
        self.closed = False
        self._connection_count = 0
    
    def connection(self):
        """Mock connection method that returns an async context manager."""
        if self.closed:
            raise psycopg.OperationalError("Pool is closed")
        
        conn = MockConnection()
        self.connections.append(conn)
        self._connection_count += 1
        return conn
    
    async def close(self):
        """Mock close method."""
        self.closed = True


class TestTimescaleWriter:
    """Test cases for TimescaleDB writer with mocked database."""
    
    @pytest.fixture
    def mock_pool(self):
        """Mock connection pool."""
        return MockConnectionPool()
    
    @pytest.fixture
    def writer_with_mock_pool(self, mock_pool):
        """Writer instance with mocked connection pool."""
        writer = TimescaleWriter(connection_string="postgresql://test:test@localhost/test")
        writer._pool = mock_pool
        writer._initialized = True
        return writer
    
    @pytest.mark.asyncio
    async def test_write_raw_event_success(self, writer_with_mock_pool):
        """Test successful raw event writing."""
        ts = 1640995200000  # 2022-01-01 00:00:00 UTC
        host_id = "192.168.1.100"
        event_type = "exec"
        payload_json = {"argv": ["/usr/bin/bash", "-c", "echo hi"]}
        
        result = await writer_with_mock_pool.write_raw_event(ts, host_id, event_type, payload_json)
        
        assert result is True
        assert len(writer_with_mock_pool._pool.connections) == 1
        
        conn = writer_with_mock_pool._pool.connections[0]
        assert conn.committed is True
        
        # Verify SQL query structure
        executed_queries = conn._cursor.executed_queries
        assert len(executed_queries) == 1
        assert "INSERT INTO events_raw" in executed_queries[0]
        assert "ts, host_id, event_type, payload_json" in executed_queries[0]
        
        # Verify parameters
        executed_params = conn._cursor.executed_params
        assert len(executed_params) == 1
        params = executed_params[0]
        assert isinstance(params[0], datetime)  # ts converted to datetime
        assert params[1] == host_id
        assert params[2] == event_type
        assert params[3] == payload_json
    
    @pytest.mark.asyncio
    async def test_write_raw_event_retry_on_operational_error(self, writer_with_mock_pool):
        """Test retry logic on operational errors."""
        ts = 1640995200000
        host_id = "192.168.1.100"
        event_type = "exec"
        payload_json = {"argv": ["/usr/bin/bash"]}
        
        # Configure mock to fail first, then succeed
        # We need to ensure there's a connection first
        await writer_with_mock_pool.write_raw_event(ts, host_id, event_type, payload_json)
        
        # Now configure the mock for retry testing
        conn = writer_with_mock_pool._pool.connections[0]
        conn._cursor.side_effects = [
            psycopg.OperationalError("Connection lost"),
            None  # Success on retry
        ]
        
        result = await writer_with_mock_pool.write_raw_event(ts, host_id, event_type, payload_json)
        
        assert result is True
        assert conn._cursor.call_count >= 2  # Failed once, succeeded on retry
    
    @pytest.mark.asyncio
    async def test_write_event_success(self, writer_with_mock_pool):
        """Test successful enriched event writing."""
        event_data = {
            "id": "test-event-001",
            "type": "exec",
            "source": "/usr/bin/bash",
            "timestamp": 1640995200000,
            "env": "dev",
            "rdns": "host-100.local",
            "metadata": {"host_id": "192.168.1.100"},
            "payload": b"test payload"
        }
        
        result = await writer_with_mock_pool.write_event(event_data)
        
        assert result is True
        assert len(writer_with_mock_pool._pool.connections) == 1
        
        conn = writer_with_mock_pool._pool.connections[0]
        assert conn.committed is True
        
        # Verify SQL query structure
        executed_queries = conn._cursor.executed_queries
        assert len(executed_queries) == 1
        assert "INSERT INTO events" in executed_queries[0]
        assert "ON CONFLICT (id) DO UPDATE" in executed_queries[0]
        
        # Verify parameters
        executed_params = conn._cursor.executed_params
        assert len(executed_params) == 1
        params = executed_params[0]
        assert params[0] == event_data["id"]
        assert params[1] == event_data["type"]
        assert params[2] == event_data["source"]
        assert params[3] == event_data["timestamp"]
        assert params[4] == event_data["env"]
        assert params[5] == event_data["rdns"]
        assert params[6] == event_data["metadata"]
        assert params[7] == event_data["payload"]
    
    @pytest.mark.asyncio
    async def test_write_events_batch_success(self, writer_with_mock_pool):
        """Test successful batch event writing."""
        events = [
            {
                "id": "test-event-001",
                "type": "exec",
                "source": "/usr/bin/bash",
                "timestamp": 1640995200000,
                "env": "dev",
                "rdns": "host-100.local",
                "metadata": {"host_id": "192.168.1.100"},
                "payload": b"test payload 1"
            },
            {
                "id": "test-event-002",
                "type": "security",
                "source": "/usr/bin/ls",
                "timestamp": 1640995201000,
                "env": "dev",
                "rdns": "host-101.local",
                "metadata": {"host_id": "192.168.1.101"},
                "payload": b"test payload 2"
            }
        ]
        
        result = await writer_with_mock_pool.write_events_batch(events)
        
        assert result == 2
        assert len(writer_with_mock_pool._pool.connections) == 1
        
        conn = writer_with_mock_pool._pool.connections[0]
        assert conn.committed is True
        
        # Verify SQL queries
        executed_queries = conn._cursor.executed_queries
        assert len(executed_queries) == 2  # One query per event
        
        for query in executed_queries:
            assert "INSERT INTO events" in query
            assert "ON CONFLICT (id) DO UPDATE" in query
    
    @pytest.mark.asyncio
    async def test_write_events_batch_with_errors(self, writer_with_mock_pool):
        """Test batch writing with some errors."""
        events = [
            {
                "id": "test-event-001",
                "type": "exec",
                "source": "/usr/bin/bash",
                "timestamp": 1640995200000,
                "env": "dev",
                "rdns": "host-100.local",
                "metadata": {"host_id": "192.168.1.100"},
                "payload": b"test payload 1"
            },
            {
                "id": "test-event-002",
                "type": "security",
                "source": "/usr/bin/ls",
                "timestamp": 1640995201000,
                "env": "dev",
                "rdns": "host-101.local",
                "metadata": {"host_id": "192.168.1.101"},
                "payload": b"test payload 2"
            }
        ]
        
        # First, ensure we have a connection
        await writer_with_mock_pool.write_events_batch(events)
        
        # Configure mock to fail on second event
        conn = writer_with_mock_pool._pool.connections[0]
        conn._cursor.side_effects = [
            None,  # First event succeeds
            Exception("Database error")  # Second event fails
        ]
        
        result = await writer_with_mock_pool.write_events_batch(events)
        
        assert result == 1  # Only first event succeeded
        assert conn._cursor.call_count >= 2  # Both events were attempted
    
    @pytest.mark.asyncio
    async def test_connection_pool_creation(self):
        """Test connection pool creation."""
        with patch('app.writers.timescale.AsyncConnectionPool') as mock_pool_class:
            mock_pool = MockConnectionPool()
            mock_pool_class.return_value = mock_pool
            
            writer = TimescaleWriter(connection_string="postgresql://test:test@localhost/test")
            await writer.connect()
            
            mock_pool_class.assert_called_once_with(
                "postgresql://test:test@localhost/test",
                min_size=1,
                max_size=10,
                kwargs={"autocommit": False}
            )
            assert writer._pool == mock_pool
    
    @pytest.mark.asyncio
    async def test_close_connection_pool(self, writer_with_mock_pool):
        """Test closing connection pool."""
        await writer_with_mock_pool.close()
        
        assert writer_with_mock_pool._pool.closed is True
    
    @pytest.mark.asyncio
    async def test_schema_initialization_sql(self, writer_with_mock_pool):
        """Test that schema initialization executes correct SQL."""
        # Reset the mock pool
        writer_with_mock_pool._pool = MockConnectionPool()
        writer_with_mock_pool._initialized = False
        
        await writer_with_mock_pool._initialize_schema()
        
        assert writer_with_mock_pool._initialized is True
        assert len(writer_with_mock_pool._pool.connections) == 1
        
        conn = writer_with_mock_pool._pool.connections[0]
        executed_queries = conn._cursor.executed_queries
        
        # Check that all expected SQL statements were executed
        sql_statements = [
            "CREATE TABLE IF NOT EXISTS events_raw",
            "CREATE TABLE IF NOT EXISTS events",
            "create_hypertable('events_raw'",
            "create_hypertable('events'",
            "CREATE INDEX IF NOT EXISTS idx_events_raw_ts",
            "CREATE INDEX IF NOT EXISTS idx_events_raw_host_id",
            "CREATE INDEX IF NOT EXISTS idx_events_raw_event_type",
            "CREATE INDEX IF NOT EXISTS idx_events_timestamp",
            "CREATE INDEX IF NOT EXISTS idx_events_type",
            "CREATE INDEX IF NOT EXISTS idx_events_env",
            "CREATE INDEX IF NOT EXISTS idx_events_metadata_host_id"
        ]
        
        for statement in sql_statements:
            assert any(statement in query for query in executed_queries), f"Missing SQL statement: {statement}"
        
        assert conn.committed is True
