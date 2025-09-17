In app/enrich.py implement:
- def enrich_event(ev: dict, env: str, fake_rdns: bool) -> dict:
  returns a NEW dict with added fields under ev["context"]:
  - context.env = env
  - context.rdns = f"host-{last_octet}.local" if fake_rdns and ev.args.dst_ip is IPv4, else None.

- Unit tests (test_enrich.py) for:
  * adds env
  * rdns added when dst_ip is 10.1.2.3 -> host-3.local
  * rdns omitted when no dst_ip or non-IPv4
