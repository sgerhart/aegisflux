"""Configuration module for ETL enrichment service."""

import os
from typing import Optional
from dotenv import load_dotenv

# Load environment variables from .env file
load_dotenv()


class Config:
    """Configuration class for ETL enrichment service."""
    
    # NATS configuration
    NATS_URL: str = os.getenv("NATS_URL", "nats://localhost:4222")
    
    # PostgreSQL/TimescaleDB configuration
    PG_HOST: str = os.getenv("PG_HOST", "localhost")
    PG_PORT: int = int(os.getenv("PG_PORT", "5432"))
    PG_DB: str = os.getenv("PG_DB", "aegisflux")
    PG_USER: str = os.getenv("PG_USER", "postgres")
    PG_PASSWORD: str = os.getenv("PG_PASSWORD", "password")
    
    # Neo4j configuration
    NEO4J_URI: str = os.getenv("NEO4J_URI", "bolt://localhost:7687")
    NEO4J_USER: str = os.getenv("NEO4J_USER", "neo4j")
    NEO4J_PASSWORD: str = os.getenv("NEO4J_PASSWORD", "password")
    
    # Application configuration
    AF_ENV: str = os.getenv("AF_ENV", "dev")
    AF_FAKE_RDNS: bool = os.getenv("AF_FAKE_RDNS", "false").lower() == "true"
    
    # NATS subjects
    RAW_EVENTS_SUBJECT: str = "events.raw"
    ENRICHED_EVENTS_SUBJECT: str = "events.enriched"
    
    # Queue group for load balancing
    ETL_QUEUE_GROUP: str = "etl"
    
    # Processing configuration
    MAX_BATCH_SIZE: int = int(os.getenv("MAX_BATCH_SIZE", "100"))
    PROCESSING_TIMEOUT: int = int(os.getenv("PROCESSING_TIMEOUT", "30"))
    
    @property
    def pg_connection_string(self) -> str:
        """Get PostgreSQL connection string."""
        return f"postgresql://{self.PG_USER}:{self.PG_PASSWORD}@{self.PG_HOST}:{self.PG_PORT}/{self.PG_DB}"
    
    @property
    def neo4j_connection_string(self) -> str:
        """Get Neo4j connection string."""
        return self.NEO4J_URI


# Global config instance
config = Config()




