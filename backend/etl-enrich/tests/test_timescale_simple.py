"""Simple tests for TimescaleDB writer core functionality."""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch
from datetime import datetime

from app.writers.timescale import TimescaleWriter


class TestTimescaleWriterSimple:
    """Simple test cases for TimescaleDB writer."""
    
    @pytest.mark.asyncio
    async def test_write_raw_event_sql_validation(self):
        """Test that write_raw_event generates correct SQL structure."""
        with patch('app.writers.timescale.AsyncConnectionPool') as mock_pool_class:
            # Create mock connection and cursor
            mock_cursor = AsyncMock()
            mock_conn = AsyncMock()
            mock_conn.cursor.return_value.__aenter__.return_value = mock_cursor
            mock_conn.cursor.return_value.__aexit__.return_value = None
            
            mock_pool = AsyncMock()
            mock_pool.connection.return_value.__aenter__.return_value = mock_conn
            mock_pool.connection.return_value.__aexit__.return_value = None
            mock_pool.closed = False
            
            mock_pool_class.return_value = mock_pool
            
            # Create writer and test
            writer = TimescaleWriter(connection_string="postgresql://test:test@localhost/test")
            writer._pool = mock_pool
            writer._initialized = True
            
            # Test write_raw_event
            ts = 1640995200000  # 2022-01-01 00:00:00 UTC
            host_id = "192.168.1.100"
            event_type = "exec"
            payload_json = {"argv": ["/usr/bin/bash", "-c", "echo hi"]}
            
            result = await writer.write_raw_event(ts, host_id, event_type, payload_json)
            
            # Verify result
            assert result is True
            
            # Verify SQL was called
            mock_cursor.execute.assert_called_once()
            call_args = mock_cursor.execute.call_args
            
            # Verify SQL structure
            sql_query = call_args[0][0]
            assert "INSERT INTO events_raw" in sql_query
            assert "ts, host_id, event_type, payload_json" in sql_query
            assert "VALUES (%s, %s, %s, %s)" in sql_query
            
            # Verify parameters
            params = call_args[0][1]
            assert len(params) == 4
            assert isinstance(params[0], datetime)  # ts converted to datetime
            assert params[1] == host_id
            assert params[2] == event_type
            assert params[3] == payload_json
            
            # Verify commit was called
            mock_conn.commit.assert_called_once()
    
    @pytest.mark.asyncio
    async def test_write_event_sql_validation(self):
        """Test that write_event generates correct SQL structure."""
        with patch('app.writers.timescale.AsyncConnectionPool') as mock_pool_class:
            # Create mock connection and cursor
            mock_cursor = AsyncMock()
            mock_conn = AsyncMock()
            mock_conn.cursor.return_value.__aenter__.return_value = mock_cursor
            mock_conn.cursor.return_value.__aexit__.return_value = None
            
            mock_pool = AsyncMock()
            mock_pool.connection.return_value.__aenter__.return_value = mock_conn
            mock_pool.connection.return_value.__aexit__.return_value = None
            mock_pool.closed = False
            
            mock_pool_class.return_value = mock_pool
            
            # Create writer and test
            writer = TimescaleWriter(connection_string="postgresql://test:test@localhost/test")
            writer._pool = mock_pool
            writer._initialized = True
            
            # Test write_event
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
            
            result = await writer.write_event(event_data)
            
            # Verify result
            assert result is True
            
            # Verify SQL was called
            mock_cursor.execute.assert_called_once()
            call_args = mock_cursor.execute.call_args
            
            # Verify SQL structure
            sql_query = call_args[0][0]
            assert "INSERT INTO events" in sql_query
            assert "ON CONFLICT (id) DO UPDATE" in sql_query
            
            # Verify parameters
            params = call_args[0][1]
            assert len(params) == 8
            assert params[0] == event_data["id"]
            assert params[1] == event_data["type"]
            assert params[2] == event_data["source"]
            assert params[3] == event_data["timestamp"]
            assert params[4] == event_data["env"]
            assert params[5] == event_data["rdns"]
            assert params[6] == event_data["metadata"]
            assert params[7] == event_data["payload"]
            
            # Verify commit was called
            mock_conn.commit.assert_called_once()
    
    @pytest.mark.asyncio
    async def test_connection_pool_creation(self):
        """Test connection pool creation with correct parameters."""
        with patch('app.writers.timescale.AsyncConnectionPool') as mock_pool_class:
            mock_pool = AsyncMock()
            mock_pool_class.return_value = mock_pool
            
            writer = TimescaleWriter(
                connection_string="postgresql://test:test@localhost/test",
                min_size=2,
                max_size=20
            )
            
            await writer.connect()
            
            # Verify pool was created with correct parameters
            mock_pool_class.assert_called_once_with(
                "postgresql://test:test@localhost/test",
                min_size=2,
                max_size=20,
                kwargs={"autocommit": False}
            )
    
    @pytest.mark.asyncio
    async def test_retry_decorator_applied(self):
        """Test that retry decorators are properly applied."""
        # Check that the methods have retry decorators
        write_raw_event_method = TimescaleWriter.write_raw_event
        write_event_method = TimescaleWriter.write_event
        
        # The methods should have retry decorators (tenacity adds attributes)
        assert hasattr(write_raw_event_method, 'retry')
        assert hasattr(write_event_method, 'retry')
    
    def test_timestamp_conversion(self):
        """Test timestamp conversion logic."""
        from datetime import datetime
        
        # Test timestamp conversion
        ts_ms = 1640995200000  # 2022-01-01 00:00:00 UTC
        ts_dt = datetime.fromtimestamp(ts_ms / 1000.0)
        
        # Verify the conversion
        expected = datetime(2022, 1, 1, 0, 0, 0)
        assert ts_dt == expected
        
        # Test with different timestamp
        ts_ms2 = 1640995260000  # 2022-01-01 00:01:00 UTC
        ts_dt2 = datetime.fromtimestamp(ts_ms2 / 1000.0)
        expected2 = datetime(2022, 1, 1, 0, 1, 0)
        assert ts_dt2 == expected2




