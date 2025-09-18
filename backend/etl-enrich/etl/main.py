import os, json, asyncio, datetime as dt
from nats.aio.client import Client as NATS

async def run():
    nc = NATS()
    await nc.connect(servers=[os.getenv("NATS_URL","nats://localhost:4222")])

    async def on_pkgmap(msg):
        data = json.loads(msg.data.decode())
        enriched = {
            "type": "pkg_cve_enriched",
            "host_id": data.get("host_id"),
            "package": data.get("package"),
            "version": data.get("version"),
            "candidates": data.get("candidates", []),
            "ts": dt.datetime.utcnow().isoformat()+"Z"
        }
        await nc.publish("etl.enriched", json.dumps(enriched).encode())
        print(f"[etl] enriched {data.get('package')}")

    await nc.subscribe("feeds.pkg.cve", cb=on_pkgmap)
    print("[etl] listening on feeds.pkg.cve -> etl.enriched")
    try:
        while True:
            await asyncio.sleep(1)
    finally:
        await nc.drain()

if __name__ == "__main__":
    asyncio.run(run())
