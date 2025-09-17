"""Event enrichment module for adding additional fields to events."""

import logging
from typing import Dict, Any, List, Optional
import ipaddress

from .config import config

logger = logging.getLogger(__name__)


def is_ipv4_address(ip_str: str) -> bool:
    """
    Check if a string is a valid IPv4 address.
    
    Args:
        ip_str: String to check
        
    Returns:
        True if it's a valid IPv4 address, False otherwise
    """
    try:
        ipaddress.IPv4Address(ip_str)
        return True
    except (ipaddress.AddressValueError, ValueError):
        return False


def extract_last_octet(ip_str: str) -> Optional[str]:
    """
    Extract the last octet from an IPv4 address.
    
    Args:
        ip_str: IPv4 address string
        
    Returns:
        Last octet as string, or None if not a valid IPv4 address
    """
    if not is_ipv4_address(ip_str):
        return None
    
    try:
        return ip_str.split('.')[-1]
    except (IndexError, AttributeError):
        return None


def enrich_event(ev: Dict[str, Any], env: str, fake_rdns: bool) -> Dict[str, Any]:
    """
    Enrich an event with additional fields under ev["context"].
    
    Args:
        ev: The event dictionary
        env: Environment string to add
        fake_rdns: Whether to generate fake reverse DNS
        
    Returns:
        NEW dictionary with added fields under ev["context"]
    """
    # Create a deep copy to avoid modifying the original
    enriched = ev.copy()
    
    # Ensure context exists
    if "context" not in enriched:
        enriched["context"] = {}
    
    # Add environment field
    enriched["context"]["env"] = env
    
    # Add fake reverse DNS if enabled and dst_ip is IPv4
    if fake_rdns:
        # Check if args.dst_ip exists and is IPv4
        args = ev.get("args", {})
        dst_ip = args.get("dst_ip")
        
        if dst_ip and is_ipv4_address(dst_ip):
            last_octet = extract_last_octet(dst_ip)
            if last_octet:
                enriched["context"]["rdns"] = f"host-{last_octet}.local"
            else:
                enriched["context"]["rdns"] = None
        else:
            enriched["context"]["rdns"] = None
    else:
        enriched["context"]["rdns"] = None
    
    return enriched


def enrich_events_batch(events: List[Dict[str, Any]], env: str = None, fake_rdns: bool = None) -> List[Dict[str, Any]]:
    """
    Enrich a batch of events.
    
    Args:
        events: List of event dictionaries
        env: Environment string (defaults to config.AF_ENV)
        fake_rdns: Whether to generate fake reverse DNS (defaults to config.AF_FAKE_RDNS)
        
    Returns:
        List of enriched event dictionaries
    """
    if env is None:
        env = config.AF_ENV
    if fake_rdns is None:
        fake_rdns = config.AF_FAKE_RDNS
    
    return [enrich_event(event, env, fake_rdns) for event in events]


def validate_enriched_event(event_data: Dict[str, Any]) -> bool:
    """
    Validate that an enriched event has the required fields.
    
    Args:
        event_data: The enriched event data
        
    Returns:
        True if valid, False otherwise
    """
    # Check that context exists
    if "context" not in event_data:
        return False
    
    context = event_data["context"]
    
    # Check required fields
    if "env" not in context:
        return False
    
    return True


# Legacy functions for backward compatibility
def generate_fake_rdns(host_id: str) -> str:
    """Generate fake reverse DNS for host_id (legacy function)."""
    try:
        # Try to parse as IP address
        ipaddress.ip_address(host_id)
        # Extract last octet from IP
        last_octet = host_id.split('.')[-1]
        return f"host-{last_octet}.local"
    except ValueError:
        # Not an IP address, use last part of host_id
        if '.' in host_id:
            last_part = host_id.split('.')[-1]
        else:
            last_part = host_id[-4:] if len(host_id) >= 4 else host_id
        return f"host-{last_part}.local"