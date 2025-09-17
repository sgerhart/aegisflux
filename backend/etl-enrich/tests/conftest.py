"""Pytest configuration and fixtures for ETL enrichment tests."""

import pytest
import asyncio
from typing import Dict, Any, List
from unittest.mock import AsyncMock, MagicMock

from app.config import config
from app.enrich import enrich_event
from app.consumer import EventConsumer
from app.publish import EnrichedEventPublisher
from app.writers.timescale import TimescaleWriter
from app.writers.neo4j import Neo4jWriter


@pytest.fixture
def sample_event() -> Dict[str, Any]:
    """Sample raw event data for testing."""
    return {
        "id": "test-event-001",
        "type": "exec",
        "source": "/usr/bin/bash",
        "timestamp": 1640995200000,
        "metadata": {
            "host_id": "192.168.1.100",
            "pid": 1234,
            "uid": 1001,
            "container_id": None,
            "binary_sha256": "abc123def456"
        },
        "payload": "eyJhcmd2IjpbIi91c3IvYmluL2Jhc2giLCItYyIsImVjaG8gaGkiXX0="
    }


@pytest.fixture
def sample_enriched_event(sample_event: Dict[str, Any]) -> Dict[str, Any]:
    """Sample enriched event data for testing."""
    return enrich_event(sample_event, env="test", fake_rdns=True)


@pytest.fixture
def sample_events_batch() -> List[Dict[str, Any]]:
    """Sample batch of events for testing."""
    return [
        {
            "id": "test-event-001",
            "type": "exec",
            "source": "/usr/bin/bash",
            "timestamp": 1640995200000,
            "metadata": {
                "host_id": "192.168.1.100",
                "pid": 1234,
                "uid": 1001
            },
            "payload": "eyJhcmd2IjpbIi91c3IvYmluL2Jhc2giLCItYyIsImVjaG8gaGkiXX0="
        },
        {
            "id": "test-event-002",
            "type": "security",
            "source": "/usr/bin/ls",
            "timestamp": 1640995201000,
            "metadata": {
                "host_id": "192.168.1.101",
                "pid": 5678,
                "uid": 1002
            },
            "payload": "eyJhcmd2IjpbIi91c3IvYmluL2xzIiwiLWxhIiwiL3RtcCJdfQ=="
        }
    ]


@pytest.fixture
def mock_nats_client():
    """Mock NATS client for testing."""
    mock_client = AsyncMock()
    mock_client.is_closed = False
    mock_client.publish = AsyncMock()
    mock_client.subscribe = AsyncMock()
    mock_client.close = AsyncMock()
    return mock_client


@pytest.fixture
def mock_timescale_connection():
    """Mock TimescaleDB connection for testing."""
    mock_conn = AsyncMock()
    mock_conn.closed = False
    mock_conn.cursor = AsyncMock()
    mock_conn.commit = AsyncMock()
    mock_conn.close = AsyncMock()
    return mock_conn


@pytest.fixture
def mock_neo4j_driver():
    """Mock Neo4j driver for testing."""
    mock_driver = AsyncMock()
    mock_driver.session = AsyncMock()
    mock_driver.close = AsyncMock()
    return mock_driver


@pytest.fixture
def event_consumer(mock_nats_client):
    """Event consumer instance for testing."""
    return EventConsumer(nats_client=mock_nats_client, batch_size=5)


@pytest.fixture
def event_publisher(mock_nats_client):
    """Event publisher instance for testing."""
    return EnrichedEventPublisher(nats_client=mock_nats_client)


@pytest.fixture
def timescale_writer(mock_timescale_connection):
    """TimescaleDB writer instance for testing."""
    writer = TimescaleWriter()
    writer._connection = mock_timescale_connection
    writer._initialized = True
    return writer


@pytest.fixture
def neo4j_writer(mock_neo4j_driver):
    """Neo4j writer instance for testing."""
    writer = Neo4jWriter()
    writer._driver = mock_neo4j_driver
    writer._initialized = True
    return writer


@pytest.fixture(scope="session")
def event_loop():
    """Create an instance of the default event loop for the test session."""
    loop = asyncio.get_event_loop_policy().new_event_loop()
    yield loop
    loop.close()


@pytest.fixture
def mock_config():
    """Mock configuration for testing."""
    return {
        "NATS_URL": "nats://localhost:4222",
        "PG_HOST": "localhost",
        "PG_PORT": 5432,
        "PG_DB": "test_db",
        "PG_USER": "test_user",
        "PG_PASSWORD": "test_password",
        "NEO4J_URI": "bolt://localhost:7687",
        "NEO4J_USER": "neo4j",
        "NEO4J_PASSWORD": "password",
        "AF_ENV": "test",
        "AF_FAKE_RDNS": True,
        "RAW_EVENTS_SUBJECT": "events.raw",
        "ENRICHED_EVENTS_SUBJECT": "events.enriched",
        "ETL_QUEUE_GROUP": "etl",
        "MAX_BATCH_SIZE": 10,
        "PROCESSING_TIMEOUT": 5
    }
