"""Event enrichment module for adding additional fields to events."""

import logging
from typing import Dict, Any, List, Optional
import ipaddress
from datetime import datetime

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


def calculate_exploitability_score(cve_data: Dict[str, Any], candidate: Dict[str, Any]) -> float:
    """
    Calculate exploitability score for a CVE candidate.
    
    Args:
        cve_data: Full CVE data from feeds.cve.updates
        candidate: CVE candidate from package mapper
        
    Returns:
        Exploitability score between 0.0 and 1.0
    """
    score = 0.0
    
    # Base score from package mapper
    base_score = candidate.get('score', 0.0)
    score += base_score * 0.4  # 40% weight for package matching score
    
    # CVSS score influence
    cvss_score = candidate.get('cvss_score', 0.0)
    if cvss_score > 0:
        # Normalize CVSS score (0-10) to 0-1
        normalized_cvss = min(cvss_score / 10.0, 1.0)
        score += normalized_cvss * 0.3  # 30% weight for CVSS score
    
    # Severity influence
    severity = candidate.get('severity', '').lower()
    severity_scores = {
        'critical': 0.3,
        'high': 0.2,
        'medium': 0.1,
        'low': 0.05
    }
    score += severity_scores.get(severity, 0.0)
    
    # CWE influence (if available)
    cwe_data = cve_data.get('cwe', {})
    if cwe_data:
        # Common high-risk CWEs
        high_risk_cwes = [
            'CWE-79',  # Cross-site Scripting
            'CWE-89',  # SQL Injection
            'CWE-78',  # OS Command Injection
            'CWE-22',  # Path Traversal
            'CWE-352', # Cross-Site Request Forgery
            'CWE-434', # Unrestricted Upload
            'CWE-502', # Deserialization
            'CWE-862', # Missing Authorization
            'CWE-863', # Incorrect Authorization
            'CWE-269', # Improper Privilege Management
        ]
        
        cwe_ids = cwe_data.get('cwe_ids', [])
        for cwe_id in cwe_ids:
            if cwe_id in high_risk_cwes:
                score += 0.1  # 10% bonus for high-risk CWEs
                break
    
    # References influence (more references = more attention)
    references = cve_data.get('references', [])
    if len(references) > 5:
        score += 0.05  # 5% bonus for well-documented CVEs
    
    # Recent CVE bonus
    published = cve_data.get('published', '')
    if published:
        try:
            pub_date = datetime.fromisoformat(published.replace('Z', '+00:00'))
            days_old = (datetime.now(pub_date.tzinfo) - pub_date).days
            if days_old < 30:  # Less than 30 days old
                score += 0.05  # 5% bonus for recent CVEs
        except:
            pass
    
    return min(score, 1.0)  # Cap at 1.0


def enrich_pkg_cve_event(pkg_cve_data: Dict[str, Any], candidate: Dict[str, Any], cve_data: Dict[str, Any]) -> Dict[str, Any]:
    """
    Enrich package CVE event by joining with CVE data.
    
    Args:
        pkg_cve_data: Package CVE mapping data from feeds.pkg.cve
        candidate: Specific CVE candidate
        cve_data: Full CVE data from feeds.cve.updates
        
    Returns:
        Enriched package CVE record
    """
    # Calculate exploitability score
    exploitability_score = calculate_exploitability_score(cve_data, candidate)
    
    # Create enriched record
    enriched_record = {
        "record_type": "pkg_cve_enriched",
        "timestamp": datetime.utcnow().isoformat() + "Z",
        "host_id": pkg_cve_data.get('host_id'),
        "package": pkg_cve_data.get('package', {}),
        "cve_candidate": {
            "cve_id": candidate.get('cve_id'),
            "score": candidate.get('score'),
            "reason": candidate.get('reason'),
            "cvss_score": candidate.get('cvss_score'),
            "severity": candidate.get('severity')
        },
        "cve_data": {
            "cve_id": cve_data.get('cve_id'),
            "published": cve_data.get('published'),
            "last_modified": cve_data.get('last_modified'),
            "descriptions": cve_data.get('descriptions', []),
            "cvss": cve_data.get('cvss', {}),
            "cwe": cve_data.get('cwe', {}),
            "affected_products": cve_data.get('affected_products', []),
            "references": cve_data.get('references', [])
        },
        "enrichment": {
            "exploitability_score": round(exploitability_score, 3),
            "risk_level": _determine_risk_level(exploitability_score),
            "enrichment_timestamp": datetime.utcnow().isoformat() + "Z",
            "enrichment_version": "1.0"
        },
        "metadata": {
            "source": "etl-enrich",
            "enrichment_pipeline": "pkg_cve_join",
            "original_timestamp": pkg_cve_data.get('timestamp'),
            "total_candidates": pkg_cve_data.get('total_candidates', 0)
        }
    }
    
    return enriched_record


def _determine_risk_level(exploitability_score: float) -> str:
    """Determine risk level based on exploitability score."""
    if exploitability_score >= 0.8:
        return "CRITICAL"
    elif exploitability_score >= 0.6:
        return "HIGH"
    elif exploitability_score >= 0.4:
        return "MEDIUM"
    elif exploitability_score >= 0.2:
        return "LOW"
    else:
        return "MINIMAL"


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