"""NATS consumer for processing raw events and package CVE data."""

import asyncio
import json
import logging
import signal
import sys
from typing import Dict, Any, Optional, List
from datetime import datetime

import nats
import orjson
from nats.aio.client import Client as NATS

from .config import config
from .enrich import enrich_event, enrich_pkg_cve_event

logger = logging.getLogger(__name__)

# Global variables for graceful shutdown
nats_client: Optional[NATS] = None
shutdown_event = asyncio.Event()


class EventConsumer:
    """NATS consumer for processing raw events and package CVE data."""
    
    def __init__(self, max_inflight: int = 100):
        self.max_inflight = max_inflight
        self.semaphore = asyncio.Semaphore(max_inflight)
        self.timescale_writer = None
        self.neo4j_writer = None
        self.nats_client = None
        self.running = False
        # CVE cache for storing CVE documents
        self.cve_cache = {}
        # Package CVE cache for storing package CVE mappings
        self.pkg_cve_cache = {}
    
    async def connect(self):
        """Connect to NATS and initialize database writers."""
        try:
            # Connect to NATS
            self.nats_client = await nats.connect(
                config.NATS_URL,
                max_reconnect_attempts=5,
                reconnect_time_wait=2
            )
            logger.info(f"Connected to NATS at {config.NATS_URL}")
            
            # Initialize database writers
            from .writers.timescale import TimescaleWriter
            from .writers.neo4j import Neo4jWriter
            
            self.timescale_writer = TimescaleWriter()
            await self.timescale_writer.connect()
            logger.info("Connected to TimescaleDB")
            
            self.neo4j_writer = Neo4jWriter()
            await self.neo4j_writer.connect()
            logger.info("Connected to Neo4j")
            
        except Exception as e:
            logger.error(f"Failed to connect: {e}")
            raise
    
    async def disconnect(self):
        """Disconnect from NATS and close database connections."""
        try:
            if self.nats_client and not self.nats_client.is_closed:
                await self.nats_client.drain()
                await self.nats_client.close()
                logger.info("Disconnected from NATS")
            
            if self.timescale_writer:
                await self.timescale_writer.close()
                logger.info("Closed TimescaleDB connection")
            
            if self.neo4j_writer:
                await self.neo4j_writer.close()
                logger.info("Closed Neo4j connection")
                
        except Exception as e:
            logger.error(f"Error during disconnect: {e}")
    
    async def _process_message(self, msg):
        """Process a single NATS message."""
        logger.info(f"🎯 RECEIVED MESSAGE: subject={msg.subject}, data_length={len(msg.data)}")
        async with self.semaphore:
            try:
                # Set per-message timeout
                await asyncio.wait_for(
                    self._handle_message(msg),
                    timeout=30.0  # 30 second timeout per message
                )
            except asyncio.TimeoutError:
                logger.error(f"Message processing timeout for subject {msg.subject}")
                # Acknowledge the message even on timeout to avoid reprocessing
                await msg.ack()
            except Exception as e:
                logger.error(f"Error processing message: {e}")
                # Acknowledge the message to avoid reprocessing
                await msg.ack()
    
    async def _handle_cve_update(self, msg):
        """Handle CVE updates from feeds.cve.updates."""
        try:
            cve_data = orjson.loads(msg.data)
            cve_id = cve_data.get('cve_id')
            if cve_id:
                self.cve_cache[cve_id] = cve_data
                logger.info(f"Cached CVE: {cve_id}")
                
                # Check if we have any pending package CVE mappings for this CVE
                await self._process_pending_pkg_cve_mappings(cve_id)
        except Exception as e:
            logger.error(f"Error handling CVE update: {e}")
    
    async def _handle_pkg_cve_mapping(self, msg):
        """Handle package CVE mappings from feeds.pkg.cve."""
        try:
            pkg_cve_data = orjson.loads(msg.data)
            host_id = pkg_cve_data.get('host_id')
            package = pkg_cve_data.get('package', {})
            package_name = package.get('name')
            candidates = pkg_cve_data.get('candidates', [])
            
            logger.info(f"Received package CVE mapping for {host_id}: {package_name} -> {len(candidates)} candidates")
            
            # Store package CVE mapping
            cache_key = f"{host_id}:{package_name}"
            self.pkg_cve_cache[cache_key] = pkg_cve_data
            
            # Process each candidate
            for candidate in candidates:
                cve_id = candidate.get('cve_id')
                if cve_id in self.cve_cache:
                    # We have the CVE data, process immediately
                    await self._process_pkg_cve_enrichment(pkg_cve_data, candidate, self.cve_cache[cve_id])
                else:
                    # Store for later processing when CVE data arrives
                    logger.debug(f"CVE {cve_id} not yet cached, will process when available")
                    
        except Exception as e:
            logger.error(f"Error handling package CVE mapping: {e}")
    
    async def _process_pending_pkg_cve_mappings(self, cve_id: str):
        """Process pending package CVE mappings when CVE data becomes available."""
        try:
            cve_data = self.cve_cache.get(cve_id)
            if not cve_data:
                return
            
            # Find all package CVE mappings that reference this CVE
            for cache_key, pkg_cve_data in self.pkg_cve_cache.items():
                candidates = pkg_cve_data.get('candidates', [])
                for candidate in candidates:
                    if candidate.get('cve_id') == cve_id:
                        await self._process_pkg_cve_enrichment(pkg_cve_data, candidate, cve_data)
                        
        except Exception as e:
            logger.error(f"Error processing pending package CVE mappings: {e}")
    
    async def _process_pkg_cve_enrichment(self, pkg_cve_data: Dict[str, Any], candidate: Dict[str, Any], cve_data: Dict[str, Any]):
        """Process package CVE enrichment by joining with CVE data."""
        try:
            # Create enriched package CVE record
            enriched_record = enrich_pkg_cve_event(pkg_cve_data, candidate, cve_data)
            
            # Publish enriched record
            enriched_json = orjson.dumps(enriched_record)
            await self.nats_client.publish(
                "etl.enriched",
                enriched_json,
                headers={
                    "x-host-id": pkg_cve_data.get('host_id'),
                    "x-package": pkg_cve_data.get('package', {}).get('name'),
                    "x-cve-id": candidate.get('cve_id'),
                    "x-enriched": "true"
                }
            )
            
            logger.info(f"Published enriched package CVE record: {candidate.get('cve_id')}")
            
        except Exception as e:
            logger.error(f"Error processing package CVE enrichment: {e}")
    
    async def _handle_message(self, msg):
        """Handle a single message with full processing pipeline."""
        logger.info("🔧 Starting message processing...")
        try:
            # 1) Parse JSON with orjson
            logger.info("🔧 Parsing JSON...")
            try:
                event_data = orjson.loads(msg.data)
                logger.info(f"🔧 JSON parsed successfully: {event_data.get('id', 'unknown')}")
            except orjson.JSONDecodeError as e:
                logger.error(f"Failed to parse JSON: {e}")
                await msg.ack()
                return
            
            # 2) Extract required fields from protobuf Event format
            logger.info("🔧 Extracting event fields...")
            event_id = event_data.get("id")
            event_type = event_data.get("type")
            source = event_data.get("source")
            timestamp = event_data.get("timestamp")
            metadata = event_data.get("metadata", {})
            payload = event_data.get("payload")
            
            # Extract host_id from metadata
            host_id = metadata.get("host_id") if isinstance(metadata, dict) else None
            logger.info(f"🔧 Extracted fields: id={event_id}, type={event_type}, host_id={host_id}")
            
            logger.info(f"🔧 Validating required fields...")
            if not all([event_id, event_type, source, timestamp]):
                logger.warning(f"Missing required fields - id: {event_id}, type: {event_type}, source: {source}, timestamp: {timestamp}")
                try:
                    await msg.ack()
                except Exception as e:
                    logger.debug(f"Message acknowledgment not needed: {e}")
                return
            logger.info(f"🔧 Field validation passed!")
            
            logger.debug(f"Processing event: {event_type} from {host_id} at {timestamp}")
            
            # 3) Write raw event to TimescaleDB
            logger.info(f"🔧 Writing to TimescaleDB...")
            try:
                # Convert timestamp to milliseconds if needed
                if isinstance(timestamp, str):
                    # Parse ISO timestamp and convert to milliseconds
                    dt = datetime.fromisoformat(timestamp.replace('Z', '+00:00'))
                    ts_ms = int(dt.timestamp() * 1000)
                else:
                    ts_ms = int(timestamp)
                
                await self.timescale_writer.write_raw_event(
                    ts=ts_ms,
                    host_id=host_id,
                    event_type=event_type,
                    payload_json=orjson.dumps(event_data).decode('utf-8')
                )
                logger.info(f"🔧 Successfully wrote to TimescaleDB: {event_type}")
            except Exception as e:
                logger.error(f"Failed to write raw event to TimescaleDB: {e}")
                # Continue processing even if raw write fails
            
            # 4) Handle connect events for graph database
            logger.info(f"🔧 Processing connect event for graph database...")
            if event_type == "connect":
                try:
                    # Extract destination information from connect event
                    # Try to parse payload as JSON for args
                    logger.info(f"🔧 Parsing payload for graph processing: {payload[:50]}...")
                    args = {}
                    if payload:
                        try:
                            import base64
                            import json
                            payload_bytes = base64.b64decode(payload)
                            logger.info(f"🔧 First decode successful, length: {len(payload_bytes)}")
                            # Try to decode again (double encoded)
                            try:
                                double_decoded = base64.b64decode(payload_bytes.decode('utf-8'))
                                args = json.loads(double_decoded.decode('utf-8'))
                                logger.info(f"🔧 Double decode successful, args: {args}")
                            except Exception as e2:
                                # Try single decode
                                args = json.loads(payload_bytes.decode('utf-8'))
                                logger.info(f"🔧 Single decode successful, args: {args}")
                        except Exception as e:
                            logger.error(f"Could not parse payload as JSON for connect event: {e}")
                    
                    dst_ip = args.get("dst_ip")
                    dst_port = args.get("dst_port")
                    logger.info(f"🔧 Extracted dst_ip: {dst_ip}, dst_port: {dst_port}")
                    
                    if dst_ip:
                        # Derive destination host ID
                        dst_host_id = self.neo4j_writer._derive_dst_host_id(dst_ip, dst_port)
                        logger.info(f"🔧 Derived dst_host_id: {dst_host_id}")
                        
                        # Upsert communication edge
                        await self.neo4j_writer.upsert_comm_edge(host_id, dst_host_id)
                        logger.info(f"🔧 Successfully upserted communication edge: {host_id} -> {dst_host_id}")
                    else:
                        logger.info(f"🔧 No dst_ip found, skipping graph processing")
                    
                except Exception as e:
                    logger.error(f"Failed to process connect event for graph: {e}")
                    # Continue processing even if graph update fails
            
            # 5) Enrich event and publish to events.enriched
            logger.info(f"🔧 Starting enrichment process...")
            try:
                # Reconstruct the original event format for enrichment
                original_event = {
                    "id": event_id,
                    "type": event_type,
                    "source": source,
                    "timestamp": timestamp,
                    "metadata": metadata,
                    "payload": payload,
                    "args": {}  # Will be populated from payload if it's JSON
                }
                
                # Try to parse payload as JSON for args
                if payload:
                    try:
                        import base64
                        import json
                        payload_bytes = base64.b64decode(payload)
                        # Try double decode first (as used by ingest service)
                        try:
                            double_decoded = base64.b64decode(payload_bytes.decode('utf-8'))
                            payload_json = json.loads(double_decoded.decode('utf-8'))
                        except Exception:
                            # Fall back to single decode
                            payload_json = json.loads(payload_bytes.decode('utf-8'))
                        original_event["args"] = payload_json
                        logger.info(f"🔧 Parsed args for enrichment: {payload_json}")
                    except Exception as e:
                        logger.debug(f"Could not parse payload as JSON: {e}")
                
                logger.info(f"🔧 Calling enrich_event...")
                enriched_event = enrich_event(
                    original_event,
                    env=config.AF_ENV,
                    fake_rdns=config.AF_FAKE_RDNS
                )
                logger.info(f"🔧 Enrichment completed successfully")
                
                # Publish enriched event
                logger.info(f"🔧 Publishing enriched event to events.enriched...")
                enriched_json = orjson.dumps(enriched_event)
                await self.nats_client.publish(
                    "events.enriched",
                    enriched_json,
                    headers={
                        "x-host-id": host_id,
                        "x-event-type": event_type,
                        "x-timestamp": str(ts_ms),
                        "x-enriched": "true"
                    }
                )
                logger.info(f"🔧 Successfully published enriched event to events.enriched")
                logger.debug(f"Published enriched event: {event_type}")
                
            except Exception as e:
                logger.error(f"Failed to enrich and publish event: {e}")
                # Continue processing even if enrichment fails
            
            # Acknowledge the message (only for JetStream messages)
            try:
                await msg.ack()
                logger.debug(f"Successfully processed event: {event_type}")
            except Exception as e:
                logger.debug(f"Message acknowledgment not needed: {e}")
                # For non-JetStream messages, we don't need to ack
            
        except Exception as e:
            logger.error(f"Unexpected error in message handling: {e}")
            try:
                await msg.ack()  # Acknowledge to avoid reprocessing
            except Exception as ack_error:
                logger.debug(f"Message acknowledgment not needed: {ack_error}")
    
    async def start(self):
        """Start consuming messages from NATS."""
        try:
            # Subscribe to events.raw WITHOUT queue group (temporarily for debugging)
            await self.nats_client.subscribe(
                "events.raw",
                cb=self._process_message
            )
            logger.info("Subscribed to events.raw WITHOUT queue group")
            
            # Subscribe to CVE updates
            await self.nats_client.subscribe(
                "feeds.cve.updates",
                cb=self._handle_cve_update
            )
            logger.info("Subscribed to feeds.cve.updates")
            
            # Subscribe to package CVE mappings
            await self.nats_client.subscribe(
                "feeds.pkg.cve",
                cb=self._handle_pkg_cve_mapping
            )
            logger.info("Subscribed to feeds.pkg.cve")
            
            self.running = True
            
            # Wait for shutdown signal
            await shutdown_event.wait()
            logger.info("Shutdown signal received")
            
        except Exception as e:
            logger.error(f"Error in consumer start: {e}")
            raise
        finally:
            self.running = False
    
    async def stop(self):
        """Stop the consumer gracefully."""
        logger.info("Stopping consumer...")
        self.running = False
        shutdown_event.set()


async def setup_signal_handlers(consumer: EventConsumer):
    """Set up signal handlers for graceful shutdown."""
    def signal_handler(signum, frame):
        logger.info(f"Received signal {signum}, initiating graceful shutdown...")
        asyncio.create_task(consumer.stop())
    
    # Register signal handlers
    signal.signal(signal.SIGTERM, signal_handler)
    signal.signal(signal.SIGINT, signal_handler)


async def main():
    """Main entry point for the consumer."""
    # Set up logging
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )
    
    consumer = None
    try:
        # Create consumer
        consumer = EventConsumer(max_inflight=100)
        
        # Set up signal handlers
        await setup_signal_handlers(consumer)
        
        # Connect to services
        await consumer.connect()
        
        # Start consuming
        logger.info("Starting ETL consumer...")
        await consumer.start()
        
    except KeyboardInterrupt:
        logger.info("Received keyboard interrupt")
    except Exception as e:
        logger.error(f"Consumer error: {e}")
        sys.exit(1)
    finally:
        if consumer:
            await consumer.disconnect()
        logger.info("Consumer stopped")


if __name__ == "__main__":
    asyncio.run(main())