"""TimescaleDB writer for time-series event data."""

import asyncio
import logging
from typing import Dict, Any, List, Optional
import psycopg
from psycopg_pool import AsyncConnectionPool
from tenacity import retry, stop_after_attempt, wait_exponential, retry_if_exception_type

from ..config import config

logger = logging.getLogger(__name__)


class TimescaleWriter:
    """Writer for storing events in TimescaleDB."""
    
    def __init__(self, connection_string: Optional[str] = None, 
                 min_size: int = 1, max_size: int = 10):
        """Initialize the TimescaleDB writer."""
        self.connection_string = connection_string or config.pg_connection_string
        self._pool: Optional[AsyncConnectionPool] = None
        self._initialized = False
        self._min_size = min_size
        self._max_size = max_size
    
    async def connect(self) -> None:
        """Connect to TimescaleDB with connection pooling."""
        try:
            self._pool = AsyncConnectionPool(
                self.connection_string,
                min_size=self._min_size,
                max_size=self._max_size,
                kwargs={"autocommit": False}
            )
            logger.info(f"Created TimescaleDB connection pool (min={self._min_size}, max={self._max_size})")
            await self._initialize_schema()
        except Exception as e:
            logger.error(f"Failed to create TimescaleDB connection pool: {e}")
            raise
    
    async def _initialize_schema(self) -> None:
        """Initialize the database schema if it doesn't exist."""
        if self._initialized:
            return
        
        try:
            async with self._pool.connection() as conn:
                async with conn.cursor() as cursor:
                    # Create events_raw table for raw event storage
                    await cursor.execute("""
                        CREATE TABLE IF NOT EXISTS events_raw (
                            ts TIMESTAMPTZ NOT NULL,
                            host_id TEXT NOT NULL,
                            event_type TEXT NOT NULL,
                            payload_json JSONB NOT NULL,
                            created_at TIMESTAMPTZ DEFAULT NOW(),
                            PRIMARY KEY (ts, host_id, event_type)
                        )
                    """)
                    
                    # Create hypertable for time-series partitioning
                    await cursor.execute("""
                        SELECT create_hypertable('events_raw', 'ts', 
                                               if_not_exists => TRUE)
                    """)
                    
                    # Create events table for enriched events
                    await cursor.execute("""
                        CREATE TABLE IF NOT EXISTS events (
                            id TEXT NOT NULL,
                            type TEXT NOT NULL,
                            source TEXT NOT NULL,
                            timestamp BIGINT NOT NULL,
                            env TEXT,
                            rdns TEXT,
                            metadata JSONB,
                            payload BYTEA,
                            created_at TIMESTAMPTZ DEFAULT NOW(),
                            PRIMARY KEY (id, created_at)
                        )
                    """)
                    
                    # Create hypertable for enriched events
                    await cursor.execute("""
                        SELECT create_hypertable('events', 'created_at', 
                                               if_not_exists => TRUE)
                    """)
                    
                    # Create indexes for common queries
                    await cursor.execute("""
                        CREATE INDEX IF NOT EXISTS idx_events_raw_ts 
                        ON events_raw (ts DESC)
                    """)
                    
                    await cursor.execute("""
                        CREATE INDEX IF NOT EXISTS idx_events_raw_host_id 
                        ON events_raw (host_id)
                    """)
                    
                    await cursor.execute("""
                        CREATE INDEX IF NOT EXISTS idx_events_raw_event_type 
                        ON events_raw (event_type)
                    """)
                    
                    await cursor.execute("""
                        CREATE INDEX IF NOT EXISTS idx_events_timestamp 
                        ON events (timestamp DESC)
                    """)
                    
                    await cursor.execute("""
                        CREATE INDEX IF NOT EXISTS idx_events_type 
                        ON events (type)
                    """)
                    
                    await cursor.execute("""
                        CREATE INDEX IF NOT EXISTS idx_events_env 
                        ON events (env)
                    """)
                    
                    await cursor.execute("""
                        CREATE INDEX IF NOT EXISTS idx_events_metadata 
                        ON events USING GIN (metadata)
                    """)
                    
                    await conn.commit()
                    self._initialized = True
                    logger.info("TimescaleDB schema initialized")
                
        except Exception as e:
            logger.error(f"Failed to initialize TimescaleDB schema: {e}")
            raise
    
    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        retry=retry_if_exception_type((psycopg.OperationalError, psycopg.InterfaceError))
    )
    async def write_raw_event(self, ts: int, host_id: str, event_type: str, payload_json: Dict[str, Any]) -> bool:
        """
        Write a raw event to the events_raw table.
        
        Args:
            ts: Unix timestamp in milliseconds
            host_id: Host identifier
            event_type: Type of event
            payload_json: Event payload as JSON dictionary
            
        Returns:
            True if written successfully, False otherwise
        """
        try:
            if not self._pool or self._pool.closed:
                await self.connect()
            
            # Convert timestamp to PostgreSQL TIMESTAMPTZ
            from datetime import datetime
            ts_dt = datetime.fromtimestamp(ts / 1000.0)
            
            async with self._pool.connection() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute("""
                        INSERT INTO events_raw (ts, host_id, event_type, payload_json)
                        VALUES (%s, %s, %s, %s)
                    """, (ts_dt, host_id, event_type, payload_json))
                    
                    await conn.commit()
                    logger.debug(f"Written raw event: {host_id}:{event_type}")
                    return True
                    
        except Exception as e:
            logger.error(f"Failed to write raw event to TimescaleDB: {e}")
            raise
    
    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        retry=retry_if_exception_type((psycopg.OperationalError, psycopg.InterfaceError))
    )
    async def write_event(self, event_data: Dict[str, Any]) -> bool:
        """
        Write a single event to TimescaleDB.
        
        Args:
            event_data: The enriched event data
            
        Returns:
            True if written successfully, False otherwise
        """
        try:
            if not self._pool or self._pool.closed:
                await self.connect()
            
            async with self._pool.connection() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute("""
                        INSERT INTO events (id, type, source, timestamp, env, rdns, metadata, payload)
                        VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
                        ON CONFLICT (id) DO UPDATE SET
                            type = EXCLUDED.type,
                            source = EXCLUDED.source,
                            timestamp = EXCLUDED.timestamp,
                            env = EXCLUDED.env,
                            rdns = EXCLUDED.rdns,
                            metadata = EXCLUDED.metadata,
                            payload = EXCLUDED.payload,
                            created_at = NOW()
                    """, (
                        event_data.get("id"),
                        event_data.get("type"),
                        event_data.get("source"),
                        event_data.get("timestamp"),
                        event_data.get("env"),
                        event_data.get("rdns"),
                        event_data.get("metadata"),
                        event_data.get("payload")
                    ))
                    
                    await conn.commit()
                    logger.debug(f"Written event {event_data.get('id')} to TimescaleDB")
                    return True
                
        except Exception as e:
            logger.error(f"Failed to write event to TimescaleDB: {e}")
            raise
    
    async def write_events_batch(self, events: List[Dict[str, Any]]) -> int:
        """
        Write a batch of events to TimescaleDB.
        
        Args:
            events: List of enriched event data
            
        Returns:
            Number of events written successfully
        """
        if not events:
            return 0
        
        try:
            if not self._pool or self._pool.closed:
                await self.connect()
            
            written_count = 0
            async with self._pool.connection() as conn:
                async with conn.cursor() as cursor:
                    for event in events:
                        try:
                            await cursor.execute("""
                                INSERT INTO events (id, type, source, timestamp, env, rdns, metadata, payload)
                                VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
                                ON CONFLICT (id) DO UPDATE SET
                                    type = EXCLUDED.type,
                                    source = EXCLUDED.source,
                                    timestamp = EXCLUDED.timestamp,
                                    env = EXCLUDED.env,
                                    rdns = EXCLUDED.rdns,
                                    metadata = EXCLUDED.metadata,
                                    payload = EXCLUDED.payload,
                                    created_at = NOW()
                            """, (
                                event.get("id"),
                                event.get("type"),
                                event.get("source"),
                                event.get("timestamp"),
                                event.get("env"),
                                event.get("rdns"),
                                event.get("metadata"),
                                event.get("payload")
                            ))
                            written_count += 1
                        except Exception as e:
                            logger.error(f"Failed to write event in batch: {e}")
                            continue
                    
                    await conn.commit()
            
            logger.info(f"Written {written_count}/{len(events)} events to TimescaleDB")
            return written_count
            
        except Exception as e:
            logger.error(f"Failed to write event batch to TimescaleDB: {e}")
            return 0
    
    async def close(self) -> None:
        """Close the TimescaleDB connection pool."""
        if self._pool and not self._pool.closed:
            await self._pool.close()
            logger.info("Closed TimescaleDB connection pool")


# Global writer instance
_writer: Optional[TimescaleWriter] = None


async def get_writer() -> TimescaleWriter:
    """Get or create the global TimescaleDB writer instance."""
    global _writer
    if _writer is None:
        _writer = TimescaleWriter()
        await _writer.connect()
    return _writer


async def write_event(event_data: Dict[str, Any]) -> bool:
    """
    Convenience function to write a single event.
    
    Args:
        event_data: The enriched event data
        
    Returns:
        True if written successfully, False otherwise
    """
    writer = await get_writer()
    return await writer.write_event(event_data)


async def write_events(events: List[Dict[str, Any]]) -> int:
    """
    Convenience function to write multiple events.
    
    Args:
        events: List of enriched event data
        
    Returns:
        Number of events written successfully
    """
    writer = await get_writer()
    return await writer.write_events_batch(events)


async def write_raw_event(ts: int, host_id: str, event_type: str, payload_json: Dict[str, Any]) -> bool:
    """
    Convenience function to write a raw event.
    
    Args:
        ts: Unix timestamp in milliseconds
        host_id: Host identifier
        event_type: Type of event
        payload_json: Event payload as JSON dictionary
        
    Returns:
        True if written successfully, False otherwise
    """
    writer = await get_writer()
    return await writer.write_raw_event(ts, host_id, event_type, payload_json)


async def close_writer() -> None:
    """Close the global writer."""
    global _writer
    if _writer:
        await _writer.close()
        _writer = None
