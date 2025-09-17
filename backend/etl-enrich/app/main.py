"""Main entry point for the ETL enrichment service."""

import asyncio
import logging
import signal
import sys
from typing import Optional

from .config import config
from .consumer import start_consumer, stop_consumer, setup_signal_handlers
from .publish import close_publisher
from .writers.timescale import close_writer as close_timescale_writer
from .writers.neo4j import close_writer as close_neo4j_writer

# Configure logging
logging.basicConfig(
    level=logging.DEBUG,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


async def main():
    """Main application entry point."""
    logger.info("Starting ETL enrichment service")
    logger.info(f"Configuration: env={config.AF_ENV}, fake_rdns={config.AF_FAKE_RDNS}")
    logger.info(f"NATS URL: {config.NATS_URL}")
    logger.info(f"TimescaleDB: {config.PG_HOST}:{config.PG_PORT}/{config.PG_DB}")
    logger.info(f"Neo4j: {config.NEO4J_URI}")
    
    try:
        # Setup signal handlers for graceful shutdown
        setup_signal_handlers()
        
        # Start the consumer
        await start_consumer()
        
        # Keep the service running
        logger.info("ETL enrichment service is running. Press Ctrl+C to stop.")
        while True:
            await asyncio.sleep(1)
            
    except KeyboardInterrupt:
        logger.info("Received interrupt signal")
    except Exception as e:
        logger.error(f"Fatal error: {e}")
        sys.exit(1)
    finally:
        # Cleanup
        logger.info("Shutting down ETL enrichment service...")
        await stop_consumer()
        await close_publisher()
        await close_timescale_writer()
        await close_neo4j_writer()
        logger.info("ETL enrichment service stopped")


if __name__ == "__main__":
    asyncio.run(main())

