import os
import json
import asyncio
import logging
import re
from datetime import datetime
from typing import Dict, List, Any, Optional, Tuple
from dataclasses import dataclass
from nats.aio.client import Client as NATS

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

@dataclass
class PackageInfo:
    """Package information from inventory."""
    name: str
    version: str
    epoch: str
    release: str
    arch: str
    distro: str
    distro_version: str

@dataclass
class CVECandidate:
    """CVE candidate with scoring."""
    cve_id: str
    score: float
    reason: str
    affected_products: List[Dict[str, Any]]
    cvss_score: Optional[float] = None
    severity: Optional[str] = None

class PackageMatcher:
    """Package matching engine with heuristics."""
    
    def __init__(self):
        self.cve_cache = {}
        self.package_patterns = {
            'openssl': ['openssl', 'libssl', 'libcrypto'],
            'curl': ['curl', 'libcurl'],
            'wget': ['wget'],
            'git': ['git'],
            'python': ['python', 'python3'],
            'nginx': ['nginx'],
            'apache': ['apache2', 'httpd'],
            'mysql': ['mysql', 'mariadb'],
            'postgresql': ['postgresql', 'postgres'],
            'redis': ['redis'],
            'nodejs': ['nodejs', 'node'],
            'java': ['openjdk', 'java'],
            'ruby': ['ruby'],
            'php': ['php'],
            'golang': ['golang', 'go'],
        }
    
    def normalize_package_name(self, name: str) -> str:
        """Normalize package name for matching."""
        # Remove common prefixes and suffixes
        name = name.lower()
        name = re.sub(r'^lib', '', name)
        name = re.sub(r'^python3?-', '', name)
        name = re.sub(r'^node-', '', name)
        name = re.sub(r'^ruby-', '', name)
        name = re.sub(r'^php-', '', name)
        name = re.sub(r'^go-', '', name)
        name = re.sub(r'^golang-', '', name)
        name = re.sub(r'^openjdk-', '', name)
        name = re.sub(r'^java-', '', name)
        return name
    
    def parse_version(self, version: str) -> Tuple[str, str, str]:
        """Parse version string into epoch:version:release format."""
        # Handle epoch:version:release format
        if ':' in version:
            parts = version.split(':', 1)
            epoch = parts[0]
            version_part = parts[1]
        else:
            epoch = ""
            version_part = version
        
        # Split version and release
        if '-' in version_part:
            version_str, release = version_part.rsplit('-', 1)
        else:
            version_str = version_part
            release = ""
        
        return epoch, version_str, release
    
    def version_compare(self, v1: str, v2: str) -> int:
        """Compare two version strings. Returns -1, 0, or 1."""
        # Simple version comparison (can be enhanced with proper semantic versioning)
        try:
            # Split by dots and compare numerically
            v1_parts = [int(x) for x in v1.split('.')]
            v2_parts = [int(x) for x in v2.split('.')]
            
            # Pad with zeros to make same length
            max_len = max(len(v1_parts), len(v2_parts))
            v1_parts.extend([0] * (max_len - len(v1_parts)))
            v2_parts.extend([0] * (max_len - len(v2_parts)))
            
            for a, b in zip(v1_parts, v2_parts):
                if a < b:
                    return -1
                elif a > b:
                    return 1
            return 0
        except:
            # Fallback to string comparison
            if v1 < v2:
                return -1
            elif v1 > v2:
                return 1
            else:
                return 0
    
    def is_vulnerable_version(self, package_version: str, affected_products: List[Dict[str, Any]]) -> bool:
        """Check if package version is vulnerable based on affected products."""
        try:
            pkg_epoch, pkg_version, pkg_release = self.parse_version(package_version)
            
            for product in affected_products:
                cpe_name = product.get('cpe_name', '')
                if not cpe_name:
                    continue
                
                # Parse CPE name: cpe:2.3:a:vendor:product:version:update:edition:language:sw_edition:target_sw:target_hw:other
                cpe_parts = cpe_name.split(':')
                if len(cpe_parts) < 6:
                    continue
                
                cpe_version = cpe_parts[5]
                if cpe_version == '*' or cpe_version == '-':
                    continue
                
                # Check version ranges
                version_start_including = product.get('version_start_including')
                version_end_including = product.get('version_end_including')
                version_start_excluding = product.get('version_start_excluding')
                version_end_excluding = product.get('version_end_excluding')
                
                # Simple version range check
                if version_start_including and self.version_compare(pkg_version, version_start_including) < 0:
                    continue
                if version_end_including and self.version_compare(pkg_version, version_end_including) > 0:
                    continue
                if version_start_excluding and self.version_compare(pkg_version, version_start_excluding) <= 0:
                    continue
                if version_end_excluding and self.version_compare(pkg_version, version_end_excluding) >= 0:
                    continue
                
                return True
        except Exception as e:
            logger.debug(f"Error checking version vulnerability: {e}")
        
        return False
    
    def calculate_heuristic_score(self, package: PackageInfo, cve_data: Dict[str, Any]) -> float:
        """Calculate heuristic score for CVE match."""
        score = 0.0
        
        # Base score for package name match
        normalized_pkg_name = self.normalize_package_name(package.name)
        
        # Check if package name matches any patterns
        for pattern, aliases in self.package_patterns.items():
            if any(alias in normalized_pkg_name for alias in aliases):
                score += 0.3
                break
        
        # Check affected products for exact matches
        affected_products = cve_data.get('affected_products', [])
        for product in affected_products:
            cpe_name = product.get('cpe_name', '').lower()
            if package.name.lower() in cpe_name:
                score += 0.4
            elif normalized_pkg_name in cpe_name:
                score += 0.3
        
        # Version vulnerability check
        if self.is_vulnerable_version(f"{package.epoch}:{package.version}:{package.release}", affected_products):
            score += 0.5
        
        # CVSS score influence
        cvss_data = cve_data.get('cvss', {})
        base_scores = cvss_data.get('base', {})
        if 'v3.1' in base_scores:
            cvss_score = base_scores['v3.1'].get('score', 0)
            score += min(cvss_score / 20.0, 0.2)  # Normalize CVSS to 0-0.2
        elif 'v3' in base_scores:
            cvss_score = base_scores['v3'].get('score', 0)
            score += min(cvss_score / 20.0, 0.2)
        elif 'v2' in base_scores:
            cvss_score = base_scores['v2'].get('score', 0)
            score += min(cvss_score / 20.0, 0.2)
        
        # Distro-specific scoring
        if package.distro.lower() in ['ubuntu', 'debian']:
            # Check for Debian/Ubuntu specific CVEs
            for product in affected_products:
                cpe_name = product.get('cpe_name', '').lower()
                if 'ubuntu' in cpe_name or 'debian' in cpe_name:
                    score += 0.1
        elif package.distro.lower() in ['rhel', 'centos', 'fedora']:
            # Check for RHEL/CentOS specific CVEs
            for product in affected_products:
                cpe_name = product.get('cpe_name', '').lower()
                if 'redhat' in cpe_name or 'centos' in cpe_name or 'fedora' in cpe_name:
                    score += 0.1
        
        return min(score, 1.0)  # Cap at 1.0
    
    def find_cve_candidates(self, package: PackageInfo, cve_data: Dict[str, Any]) -> List[CVECandidate]:
        """Find CVE candidates for a package."""
        candidates = []
        
        # Check if package is vulnerable
        if not self.is_vulnerable_version(f"{package.epoch}:{package.version}:{package.release}", 
                                        cve_data.get('affected_products', [])):
            return candidates
        
        # Calculate score
        score = self.calculate_heuristic_score(package, cve_data)
        
        if score > 0.1:  # Only include candidates with meaningful scores
            cve_id = cve_data.get('cve_id', '')
            affected_products = cve_data.get('affected_products', [])
            
            # Get CVSS info
            cvss_data = cve_data.get('cvss', {})
            base_scores = cvss_data.get('base', {})
            cvss_score = None
            severity = None
            
            if 'v3.1' in base_scores:
                cvss_score = base_scores['v3.1'].get('score')
                severity = base_scores['v3.1'].get('severity')
            elif 'v3' in base_scores:
                cvss_score = base_scores['v3'].get('score')
                severity = base_scores['v3'].get('severity')
            elif 'v2' in base_scores:
                cvss_score = base_scores['v2'].get('score')
                severity = base_scores['v2'].get('severity')
            
            reason = f"Package {package.name} version {package.version} matches affected products"
            if cvss_score:
                reason += f" (CVSS: {cvss_score})"
            
            candidates.append(CVECandidate(
                cve_id=cve_id,
                score=score,
                reason=reason,
                affected_products=affected_products,
                cvss_score=cvss_score,
                severity=severity
            ))
        
        return candidates

class PackageMapper:
    """Main package mapper service."""
    
    def __init__(self):
        self.matcher = PackageMatcher()
        self.cve_cache = {}
        self.nc = None
    
    async def start(self, nats_url: str):
        """Start the package mapper service."""
        self.nc = NATS()
        await self.nc.connect(servers=[nats_url])
        
        # Subscribe to inventory.packages
        await self.nc.subscribe("inventory.packages", cb=self.handle_inventory)
        logger.info("Subscribed to inventory.packages")
        
        # Subscribe to CVE updates to maintain cache
        await self.nc.subscribe("feeds.cve.updates", cb=self.handle_cve_update)
        logger.info("Subscribed to feeds.cve.updates")
    
    async def handle_cve_update(self, msg):
        """Handle CVE updates to maintain cache."""
        try:
            cve_data = json.loads(msg.data.decode())
            cve_id = cve_data.get('cve_id')
            if cve_id:
                self.cve_cache[cve_id] = cve_data
                logger.debug(f"Cached CVE: {cve_id}")
        except Exception as e:
            logger.error(f"Error handling CVE update: {e}")
    
    async def handle_inventory(self, msg):
        """Handle inventory.packages messages."""
        try:
            inventory = json.loads(msg.data.decode())
            host_id = inventory.get('host_id', 'unknown')
            distro_info = inventory.get('distro', {})
            distro_name = distro_info.get('name', 'unknown')
            distro_version = distro_info.get('version', 'unknown')
            packages = inventory.get('packages', [])
            
            logger.info(f"Processing inventory for {host_id} ({distro_name} {distro_version}): {len(packages)} packages")
            
            # Process each package
            for pkg_data in packages:
                await self.process_package(host_id, distro_name, distro_version, pkg_data)
                
        except Exception as e:
            logger.error(f"Error handling inventory: {e}")
    
    async def process_package(self, host_id: str, distro: str, distro_version: str, pkg_data: Dict[str, Any]):
        """Process a single package and find CVE candidates."""
        try:
            # Create PackageInfo object
            package = PackageInfo(
                name=pkg_data.get('name', ''),
                version=pkg_data.get('version', ''),
                epoch=pkg_data.get('epoch', ''),
                release=pkg_data.get('release', ''),
                arch=pkg_data.get('arch', ''),
                distro=distro,
                distro_version=distro_version
            )
            
            # Find CVE candidates
            all_candidates = []
            for cve_id, cve_data in self.cve_cache.items():
                candidates = self.matcher.find_cve_candidates(package, cve_data)
                all_candidates.extend(candidates)
            
            # Sort by score (highest first)
            all_candidates.sort(key=lambda x: x.score, reverse=True)
            
            # Take top 10 candidates
            top_candidates = all_candidates[:10]
            
            if top_candidates:
                # Create mapping message
                mapping = {
                    "host_id": host_id,
                    "package": {
                        "name": package.name,
                        "version": package.version,
                        "epoch": package.epoch,
                        "release": package.release,
                        "arch": package.arch,
                        "distro": package.distro,
                        "distro_version": package.distro_version
                    },
                    "candidates": [
                        {
                            "cve_id": c.cve_id,
                            "score": round(c.score, 3),
                            "reason": c.reason,
                            "cvss_score": c.cvss_score,
                            "severity": c.severity
                        }
                        for c in top_candidates
                    ],
                    "timestamp": datetime.utcnow().isoformat() + "Z",
                    "total_candidates": len(all_candidates)
                }
                
                # Publish mapping
                await self.nc.publish("feeds.pkg.cve", json.dumps(mapping).encode())
                logger.info(f"Mapped {package.name}-{package.version} -> {len(top_candidates)} CVE candidates")
                
                # Log top candidate
                if top_candidates:
                    top_candidate = top_candidates[0]
                    logger.info(f"  Top candidate: {top_candidate.cve_id} (score: {top_candidate.score:.3f})")
            else:
                logger.debug(f"No CVE candidates found for {package.name}-{package.version}")
                
        except Exception as e:
            logger.error(f"Error processing package {pkg_data.get('name', 'unknown')}: {e}")
    
    async def stop(self):
        """Stop the package mapper service."""
        if self.nc:
            await self.nc.drain()

async def run():
    """Main execution function."""
    nats_url = os.getenv("NATS_URL", "nats://localhost:4222")
    
    logger.info("Starting Package Mapper service...")
    logger.info(f"NATS URL: {nats_url}")
    
    mapper = PackageMapper()
    
    try:
        await mapper.start(nats_url)
        logger.info("Package Mapper service started successfully")
        
        # Keep running
        while True:
            await asyncio.sleep(1)
            
    except KeyboardInterrupt:
        logger.info("Shutting down Package Mapper service...")
    except Exception as e:
        logger.error(f"Package Mapper service error: {e}")
    finally:
        await mapper.stop()

if __name__ == "__main__":
    asyncio.run(run())
