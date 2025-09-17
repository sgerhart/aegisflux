"""Tests for database writers."""

import pytest
from unittest.mock import AsyncMock, MagicMock
from app.writers.timescale import TimescaleWriter
from app.writers.neo4j import Neo4jWriter


class TestTimescaleWriter:
    """Test cases for TimescaleDB writer."""
    
    @pytest.mark.asyncio
    async def test_write_event_success(self, timescale_writer, sample_enriched_event):
        """Test successful event writing to TimescaleDB."""
        result = await timescale_writer.write_event(sample_enriched_event)
        
        assert result is True
        timescale_writer._connection.cursor.assert_called_once()
    
    @pytest.mark.asyncio
    async def test_write_event_connection_error(self, sample_enriched_event):
        """Test event writing with connection error."""
        writer = TimescaleWriter()
        writer._connection = None
        
        with pytest.raises(Exception):
            await writer.write_event(sample_enriched_event)
    
    @pytest.mark.asyncio
    async def test_write_events_batch_success(self, timescale_writer, sample_events_batch):
        """Test successful batch writing to TimescaleDB."""
        # Enrich the events first
        enriched_events = []
        for event in sample_events_batch:
            enriched = event.copy()
            enriched["env"] = "test"
            enriched["metadata"] = event.get("metadata", {})
            enriched["metadata"]["enriched_at"] = 1640995200000
            enriched_events.append(enriched)
        
        result = await timescale_writer.write_events_batch(enriched_events)
        
        assert result == len(enriched_events)
        timescale_writer._connection.cursor.assert_called_once()
    
    @pytest.mark.asyncio
    async def test_write_events_batch_empty(self, timescale_writer):
        """Test batch writing with empty list."""
        result = await timescale_writer.write_events_batch([])
        
        assert result == 0
        timescale_writer._connection.cursor.assert_not_called()
    
    @pytest.mark.asyncio
    async def test_connect_success(self):
        """Test successful connection to TimescaleDB."""
        mock_conn = AsyncMock()
        mock_conn.cursor = AsyncMock()
        mock_conn.commit = AsyncMock()
        
        with pytest.MonkeyPatch().context() as m:
            m.setattr("psycopg.AsyncConnection.connect", AsyncMock(return_value=mock_conn))
            writer = TimescaleWriter()
            await writer.connect()
            
            assert writer._connection == mock_conn
            assert writer._initialized is True
    
    @pytest.mark.asyncio
    async def test_close_connection(self, timescale_writer):
        """Test closing TimescaleDB connection."""
        await timescale_writer.close()
        
        timescale_writer._connection.close.assert_called_once()


class TestNeo4jWriter:
    """Test cases for Neo4j writer."""
    
    @pytest.mark.asyncio
    async def test_write_event_success(self, neo4j_writer, sample_enriched_event):
        """Test successful event writing to Neo4j."""
        result = await neo4j_writer.write_event(sample_enriched_event)
        
        assert result is True
        neo4j_writer._driver.session.assert_called_once()
    
    @pytest.mark.asyncio
    async def test_write_event_connection_error(self, sample_enriched_event):
        """Test event writing with connection error."""
        writer = Neo4jWriter()
        writer._driver = None
        
        with pytest.raises(Exception):
            await writer.write_event(sample_enriched_event)
    
    @pytest.mark.asyncio
    async def test_write_events_batch_success(self, neo4j_writer, sample_events_batch):
        """Test successful batch writing to Neo4j."""
        # Enrich the events first
        enriched_events = []
        for event in sample_events_batch:
            enriched = event.copy()
            enriched["env"] = "test"
            enriched["rdns"] = "host-100.local"
            enriched["metadata"] = event.get("metadata", {})
            enriched["metadata"]["enriched_at"] = 1640995200000
            enriched_events.append(enriched)
        
        result = await neo4j_writer.write_events_batch(enriched_events)
        
        assert result == len(enriched_events)
        neo4j_writer._driver.session.assert_called()
    
    @pytest.mark.asyncio
    async def test_write_events_batch_empty(self, neo4j_writer):
        """Test batch writing with empty list."""
        result = await neo4j_writer.write_events_batch([])
        
        assert result == 0
        neo4j_writer._driver.session.assert_not_called()
    
    @pytest.mark.asyncio
    async def test_connect_success(self):
        """Test successful connection to Neo4j."""
        mock_driver = AsyncMock()
        mock_driver.verify_connectivity = AsyncMock()
        mock_driver.session = AsyncMock()
        mock_driver.close = AsyncMock()
        
        with pytest.MonkeyPatch().context() as m:
            m.setattr("neo4j.AsyncGraphDatabase.driver", MagicMock(return_value=mock_driver))
            writer = Neo4jWriter()
            await writer.connect()
            
            assert writer._driver == mock_driver
            assert writer._initialized is True
    
    @pytest.mark.asyncio
    async def test_close_connection(self, neo4j_writer):
        """Test closing Neo4j connection."""
        await neo4j_writer.close()
        
        neo4j_writer._driver.close.assert_called_once()
    
    @pytest.mark.asyncio
    async def test_create_event_relationships_exec(self, neo4j_writer):
        """Test creating relationships for exec events."""
        mock_session = AsyncMock()
        event_data = {
            "id": "test-event-001",
            "type": "exec",
            "metadata": {
                "host_id": "192.168.1.100",
                "pid": 1234,
                "uid": 1001
            },
            "source": "/usr/bin/bash"
        }
        
        await neo4j_writer._create_event_relationships(mock_session, event_data)
        
        # Should create user and process relationships
        assert mock_session.run.call_count >= 2
    
    @pytest.mark.asyncio
    async def test_create_event_relationships_with_container(self, neo4j_writer):
        """Test creating relationships for events with container."""
        mock_session = AsyncMock()
        event_data = {
            "id": "test-event-001",
            "type": "exec",
            "metadata": {
                "host_id": "192.168.1.100",
                "pid": 1234,
                "uid": 1001,
                "container_id": "container-123"
            },
            "source": "/usr/bin/bash"
        }
        
        await neo4j_writer._create_event_relationships(mock_session, event_data)
        
        # Should create user, process, and container relationships
        assert mock_session.run.call_count >= 3


class TestWriterIntegration:
    """Integration tests for writers."""
    
    @pytest.mark.asyncio
    async def test_timescale_writer_retry_mechanism(self, sample_enriched_event):
        """Test TimescaleDB writer retry mechanism."""
        mock_conn = AsyncMock()
        mock_conn.closed = False
        mock_conn.cursor = AsyncMock()
        mock_conn.commit = AsyncMock()
        
        # First call raises exception, second succeeds
        mock_cursor = AsyncMock()
        mock_cursor.execute = AsyncMock(side_effect=[Exception("Connection error"), None])
        mock_conn.cursor.return_value.__aenter__.return_value = mock_cursor
        
        writer = TimescaleWriter()
        writer._connection = mock_conn
        writer._initialized = True
        
        # Should retry and eventually succeed
        result = await writer.write_event(sample_enriched_event)
        assert result is True
    
    @pytest.mark.asyncio
    async def test_neo4j_writer_retry_mechanism(self, sample_enriched_event):
        """Test Neo4j writer retry mechanism."""
        mock_driver = AsyncMock()
        mock_session = AsyncMock()
        mock_driver.session.return_value.__aenter__.return_value = mock_session
        
        # First call raises exception, second succeeds
        mock_session.run = AsyncMock(side_effect=[Exception("Connection error"), None])
        
        writer = Neo4jWriter()
        writer._driver = mock_driver
        writer._initialized = True
        
        # Should retry and eventually succeed
        result = await writer.write_event(sample_enriched_event)
        assert result is True

