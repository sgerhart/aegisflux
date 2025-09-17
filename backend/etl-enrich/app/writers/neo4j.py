"""Neo4j writer for graph event data."""

import asyncio
import logging
from typing import Dict, Any, List, Optional, Union
import neo4j
from neo4j import Driver, Session, Transaction
from tenacity import retry, stop_after_attempt, wait_exponential, retry_if_exception_type

from ..config import config

logger = logging.getLogger(__name__)


class Neo4jWriter:
    """Writer for storing events and relationships in Neo4j."""
    
    def __init__(self, uri: Optional[str] = None, 
                 username: Optional[str] = None, 
                 password: Optional[str] = None):
        """Initialize the Neo4j writer."""
        self.uri = uri or config.NEO4J_URI
        self.username = username or config.NEO4J_USER
        self.password = password or config.NEO4J_PASSWORD
        self._driver: Optional[Driver] = None
        self._initialized = False
    
    async def connect(self) -> Driver:
        """Connect to Neo4j and return the driver."""
        try:
            self._driver = neo4j.GraphDatabase.driver(
                self.uri,
                auth=(self.username, self.password)
            )
            # Verify connectivity
            with self._driver.session() as session:
                session.run("RETURN 1")
            logger.info("Connected to Neo4j")
            await self._initialize_schema()
            return self._driver
        except Exception as e:
            logger.error(f"Failed to connect to Neo4j: {e}")
            raise
    
    async def _initialize_schema(self) -> None:
        """Initialize the Neo4j schema with constraints and indexes."""
        if self._initialized:
            return
        
        try:
            with self._driver.session() as session:
                # Create constraints
                session.run("""
                    CREATE CONSTRAINT host_id_unique IF NOT EXISTS
                    FOR (h:Host) REQUIRE h.host_id IS UNIQUE
                """)
                
                session.run("""
                    CREATE CONSTRAINT network_endpoint_id_unique IF NOT EXISTS
                    FOR (n:NetworkEndpoint) REQUIRE n.endpoint_id IS UNIQUE
                """)
                
                # Create indexes
                session.run("""
                    CREATE INDEX host_rdns_index IF NOT EXISTS
                    FOR (h:Host) ON (h.rdns)
                """)
                
                session.run("""
                    CREATE INDEX network_endpoint_ip_index IF NOT EXISTS
                    FOR (n:NetworkEndpoint) ON (n.ip)
                """)
                
                self._initialized = True
                logger.info("Neo4j schema initialized")
                
        except Exception as e:
            logger.error(f"Failed to initialize Neo4j schema: {e}")
            raise
    
    def _derive_dst_host_id(self, dst_ip: str, dst_port: int) -> str:
        """
        Derive destination host_id from IP address.
        
        For MVP, if it's an internal known host, use the host_id.
        Otherwise, create as NetworkEndpoint with format "ip:<dst_ip>".
        
        Args:
            dst_ip: Destination IP address
            dst_port: Destination port
            
        Returns:
            Host ID or NetworkEndpoint ID
        """
        # For MVP, assume all external IPs are network endpoints
        # In a real implementation, you'd check against known internal hosts
        internal_networks = [
            "192.168.", "10.", "172.16.", "172.17.", "172.18.", "172.19.",
            "172.20.", "172.21.", "172.22.", "172.23.", "172.24.", "172.25.",
            "172.26.", "172.27.", "172.28.", "172.29.", "172.30.", "172.31."
        ]
        
        # Check if it's an internal IP
        is_internal = any(dst_ip.startswith(net) for net in internal_networks)
        
        if is_internal:
            # For internal IPs, try to derive host_id
            # In a real implementation, you'd have a mapping service
            return f"host-{dst_ip.replace('.', '-')}"
        else:
            # For external IPs, create as NetworkEndpoint
            return f"ip:{dst_ip}:{dst_port}"
    
    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        retry=retry_if_exception_type((neo4j.exceptions.ServiceUnavailable, neo4j.exceptions.TransientError))
    )
    async def upsert_comm_edge(self, src_host_id: str, dst_host_id: str) -> bool:
        """
        Upsert a communication edge between two hosts.
        
        Args:
            src_host_id: Source host identifier
            dst_host_id: Destination host identifier
            
        Returns:
            True if successful, False otherwise
        """
        try:
            if not self._driver:
                await self.connect()
            
            with self._driver.session() as session:
                # Determine if dst is a Host or NetworkEndpoint
                if dst_host_id.startswith("ip:"):
                    # It's a NetworkEndpoint
                    endpoint_id = dst_host_id
                    ip_port = dst_host_id[3:]  # Remove "ip:" prefix
                    ip, port = ip_port.split(":") if ":" in ip_port else (ip_port, "0")
                    
                    # Upsert NetworkEndpoint and relationship
                    session.run("""
                        MERGE (a:Host {host_id: $src})
                        MERGE (b:NetworkEndpoint {endpoint_id: $dst})
                        SET b.ip = $ip, b.port = $port
                        MERGE (a)-[r:COMMUNICATES]->(b)
                        ON CREATE SET r.count_1h = 1, r.last_seen = timestamp()
                        ON MATCH SET r.count_1h = coalesce(r.count_1h, 0) + 1, r.last_seen = timestamp()
                    """, {
                        "src": src_host_id,
                        "dst": dst_host_id,
                        "ip": ip,
                        "port": int(port)
                    })
                else:
                    # It's a Host
                    session.run("""
                        MERGE (a:Host {host_id: $src})
                        MERGE (b:Host {host_id: $dst})
                        MERGE (a)-[r:COMMUNICATES]->(b)
                        ON CREATE SET r.count_1h = 1, r.last_seen = timestamp()
                        ON MATCH SET r.count_1h = coalesce(r.count_1h, 0) + 1, r.last_seen = timestamp()
                    """, {
                        "src": src_host_id,
                        "dst": dst_host_id
                    })
                
                logger.debug(f"Upserted communication edge: {src_host_id} -> {dst_host_id}")
                return True
                
        except Exception as e:
            logger.error(f"Failed to upsert communication edge: {e}")
            raise
    
    def _parse_connect_event(self, event_data: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        """
        Parse a connect event to extract connection information.
        
        Args:
            event_data: The event data
            
        Returns:
            Dictionary with src_host_id, dst_host_id, dst_ip, dst_port if it's a connect event
            None if it's not a connect event or missing required fields
        """
        # Check if it's a connect event
        if event_data.get("type") != "connect":
            return None
        
        # Extract source host_id
        src_host_id = event_data.get("metadata", {}).get("host_id")
        if not src_host_id:
            logger.warning("Connect event missing source host_id")
            return None
        
        # Parse payload to extract connection details
        payload = event_data.get("payload")
        if not payload:
            logger.warning("Connect event missing payload")
            return None
        
        try:
            import orjson
            if isinstance(payload, bytes):
                payload_data = orjson.loads(payload)
            else:
                payload_data = payload
            
            # Extract connection details from args
            args = payload_data.get("args", {})
            dst_ip = args.get("dst_ip")
            dst_port = args.get("dst_port")
            
            if not dst_ip or dst_port is None:
                logger.warning(f"Connect event missing dst_ip or dst_port: {args}")
                return None
            
            # Derive destination host_id
            dst_host_id = self._derive_dst_host_id(dst_ip, dst_port)
            
            return {
                "src_host_id": src_host_id,
                "dst_host_id": dst_host_id,
                "dst_ip": dst_ip,
                "dst_port": dst_port
            }
            
        except Exception as e:
            logger.error(f"Failed to parse connect event payload: {e}")
            return None
    
    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        retry=retry_if_exception_type((neo4j.exceptions.ServiceUnavailable, neo4j.exceptions.TransientError))
    )
    async def write_event(self, event_data: Dict[str, Any]) -> bool:
        """
        Write a single event to Neo4j.
        
        Args:
            event_data: The enriched event data
            
        Returns:
            True if written successfully, False otherwise
        """
        try:
            if not self._driver:
                await self.connect()
            
            with self._driver.session() as session:
                # Create event node
                session.run("""
                    MERGE (e:Event {id: $event_id})
                    SET e.type = $type,
                        e.source = $source,
                        e.timestamp = $timestamp,
                        e.env = $env,
                        e.rdns = $rdns,
                        e.metadata = $metadata,
                        e.payload = $payload,
                        e.created_at = datetime()
                """, {
                    "event_id": event_data.get("id"),
                    "type": event_data.get("type"),
                    "source": event_data.get("source"),
                    "timestamp": event_data.get("timestamp"),
                    "env": event_data.get("env"),
                    "rdns": event_data.get("rdns"),
                    "metadata": event_data.get("metadata", {}),
                    "payload": event_data.get("payload")
                })
                
                # Create host node and relationship
                host_id = event_data.get("metadata", {}).get("host_id")
                if host_id:
                    session.run("""
                        MERGE (h:Host {host_id: $host_id})
                        SET h.rdns = $rdns,
                            h.env = $env,
                            h.last_seen = datetime()
                        MERGE (h)-[:GENERATED]->(e:Event {id: $event_id})
                    """, {
                        "host_id": host_id,
                        "rdns": event_data.get("rdns"),
                        "env": event_data.get("env"),
                        "event_id": event_data.get("id")
                    })
                
                # Handle connect events specifically
                connect_info = self._parse_connect_event(event_data)
                if connect_info:
                    await self.upsert_comm_edge(
                        connect_info["src_host_id"],
                        connect_info["dst_host_id"]
                    )
                
                # Create other relationships based on event type
                await self._create_event_relationships(session, event_data)
                
                logger.debug(f"Written event {event_data.get('id')} to Neo4j")
                return True
                
        except Exception as e:
            logger.error(f"Failed to write event to Neo4j: {e}")
            raise
    
    async def _create_event_relationships(self, session: Session, event_data: Dict[str, Any]) -> None:
        """Create relationships based on event type and metadata."""
        event_id = event_data.get("id")
        event_type = event_data.get("type")
        metadata = event_data.get("metadata", {})
        
        # Create user relationship for exec events
        if event_type == "exec" and "uid" in metadata:
            session.run("""
                MERGE (u:User {uid: $uid})
                SET u.last_seen = datetime()
                MERGE (u)-[:EXECUTED]->(e:Event {id: $event_id})
            """, {
                "uid": metadata["uid"],
                "event_id": event_id
            })
        
        # Create process relationship
        if "pid" in metadata:
            session.run("""
                MERGE (p:Process {pid: $pid, host_id: $host_id})
                SET p.binary_path = $binary_path,
                    p.last_seen = datetime()
                MERGE (p)-[:EXECUTED]->(e:Event {id: $event_id})
            """, {
                "pid": metadata["pid"],
                "host_id": metadata.get("host_id"),
                "binary_path": event_data.get("source"),
                "event_id": event_id
            })
        
        # Create container relationship
        container_id = metadata.get("container_id")
        if container_id:
            session.run("""
                MERGE (c:Container {container_id: $container_id})
                SET c.last_seen = datetime()
                MERGE (c)-[:GENERATED]->(e:Event {id: $event_id})
            """, {
                "container_id": container_id,
                "event_id": event_id
            })
    
    async def write_events_batch(self, events: List[Dict[str, Any]]) -> int:
        """
        Write a batch of events to Neo4j.
        
        Args:
            events: List of enriched event data
            
        Returns:
            Number of events written successfully
        """
        if not events:
            return 0
        
        try:
            if not self._driver:
                await self.connect()
            
            written_count = 0
            with self._driver.session() as session:
                for event in events:
                    try:
                        # Create event node
                        session.run("""
                            MERGE (e:Event {id: $event_id})
                            SET e.type = $type,
                                e.source = $source,
                                e.timestamp = $timestamp,
                                e.env = $env,
                                e.rdns = $rdns,
                                e.metadata = $metadata,
                                e.payload = $payload,
                                e.created_at = datetime()
                        """, {
                            "event_id": event.get("id"),
                            "type": event.get("type"),
                            "source": event.get("source"),
                            "timestamp": event.get("timestamp"),
                            "env": event.get("env"),
                            "rdns": event.get("rdns"),
                            "metadata": event.get("metadata", {}),
                            "payload": event.get("payload")
                        })
                        
                        # Create host relationship
                        host_id = event.get("metadata", {}).get("host_id")
                        if host_id:
                            session.run("""
                                MERGE (h:Host {host_id: $host_id})
                                SET h.rdns = $rdns,
                                    h.env = $env,
                                    h.last_seen = datetime()
                                MERGE (h)-[:GENERATED]->(e:Event {id: $event_id})
                            """, {
                                "host_id": host_id,
                                "rdns": event.get("rdns"),
                                "env": event.get("env"),
                                "event_id": event.get("id")
                            })
                        
                        # Handle connect events
                        connect_info = self._parse_connect_event(event)
                        if connect_info:
                            await self.upsert_comm_edge(
                                connect_info["src_host_id"],
                                connect_info["dst_host_id"]
                            )
                        
                        # Create other relationships
                        await self._create_event_relationships(session, event)
                        
                        written_count += 1
                    except Exception as e:
                        logger.error(f"Failed to write event in batch: {e}")
                        continue
            
            logger.info(f"Written {written_count}/{len(events)} events to Neo4j")
            return written_count
            
        except Exception as e:
            logger.error(f"Failed to write event batch to Neo4j: {e}")
            return 0
    
    async def close(self) -> None:
        """Close the Neo4j connection."""
        if self._driver:
            self._driver.close()
            logger.info("Closed Neo4j connection")


# Global writer instance
_writer: Optional[Neo4jWriter] = None


async def get_writer() -> Neo4jWriter:
    """Get or create the global Neo4j writer instance."""
    global _writer
    if _writer is None:
        _writer = Neo4jWriter()
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


async def upsert_comm_edge(src_host_id: str, dst_host_id: str) -> bool:
    """
    Convenience function to upsert a communication edge.
    
    Args:
        src_host_id: Source host identifier
        dst_host_id: Destination host identifier
        
    Returns:
        True if successful, False otherwise
    """
    writer = await get_writer()
    return await writer.upsert_comm_edge(src_host_id, dst_host_id)


async def close_writer() -> None:
    """Close the global writer."""
    global _writer
    if _writer:
        await _writer.close()
        _writer = None