"""Event publishing module for enriched events."""

import asyncio
import logging
from typing import Dict, Any, List, Optional
import nats
import orjson
from tenacity import retry, stop_after_attempt, wait_exponential

from .config import config

logger = logging.getLogger(__name__)


class EnrichedEventPublisher:
    """Publisher for enriched events to NATS."""
    
    def __init__(self, nats_client: Optional[nats.NATS] = None):
        """Initialize the publisher."""
        self.nats_client = nats_client
        self._connection_retries = 0
        self._max_retries = 5
    
    async def connect(self) -> None:
        """Connect to NATS if not already connected."""
        if self.nats_client is None or self.nats_client.is_closed:
            try:
                self.nats_client = await nats.connect(config.NATS_URL)
                logger.info(f"Connected to NATS at {config.NATS_URL}")
                self._connection_retries = 0
            except Exception as e:
                self._connection_retries += 1
                logger.error(f"Failed to connect to NATS (attempt {self._connection_retries}): {e}")
                if self._connection_retries >= self._max_retries:
                    raise
                await asyncio.sleep(2 ** self._connection_retries)
                await self.connect()
    
    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10)
    )
    async def publish_event(self, event_data: Dict[str, Any]) -> bool:
        """
        Publish a single enriched event.
        
        Args:
            event_data: The enriched event data
            
        Returns:
            True if published successfully, False otherwise
        """
        try:
            await self.connect()
            
            # Serialize event to JSON
            event_json = orjson.dumps(event_data)
            
            # Publish to enriched events subject
            await self.nats_client.publish(
                config.ENRICHED_EVENTS_SUBJECT,
                event_json
            )
            
            logger.debug(f"Published enriched event: {event_data.get('id', 'unknown')}")
            return True
            
        except Exception as e:
            logger.error(f"Failed to publish event: {e}")
            raise
    
    async def publish_events_batch(self, events: List[Dict[str, Any]]) -> int:
        """
        Publish a batch of enriched events.
        
        Args:
            events: List of enriched event data
            
        Returns:
            Number of events published successfully
        """
        if not events:
            return 0
        
        try:
            await self.connect()
            
            published_count = 0
            for event in events:
                try:
                    await self.publish_event(event)
                    published_count += 1
                except Exception as e:
                    logger.error(f"Failed to publish event in batch: {e}")
                    continue
            
            logger.info(f"Published {published_count}/{len(events)} enriched events")
            return published_count
            
        except Exception as e:
            logger.error(f"Failed to publish event batch: {e}")
            return 0
    
    async def close(self) -> None:
        """Close the NATS connection."""
        if self.nats_client and not self.nats_client.is_closed:
            await self.nats_client.close()
            logger.info("Closed NATS connection")


# Global publisher instance
_publisher: Optional[EnrichedEventPublisher] = None


async def get_publisher() -> EnrichedEventPublisher:
    """Get or create the global publisher instance."""
    global _publisher
    if _publisher is None:
        _publisher = EnrichedEventPublisher()
        await _publisher.connect()
    return _publisher


async def publish_enriched_event(event_data: Dict[str, Any]) -> bool:
    """
    Convenience function to publish a single enriched event.
    
    Args:
        event_data: The enriched event data
        
    Returns:
        True if published successfully, False otherwise
    """
    publisher = await get_publisher()
    return await publisher.publish_event(event_data)


async def publish_enriched_events(events: List[Dict[str, Any]]) -> int:
    """
    Convenience function to publish multiple enriched events.
    
    Args:
        events: List of enriched event data
        
    Returns:
        Number of events published successfully
    """
    publisher = await get_publisher()
    return await publisher.publish_events_batch(events)


async def close_publisher() -> None:
    """Close the global publisher."""
    global _publisher
    if _publisher:
        await _publisher.close()
        _publisher = None

