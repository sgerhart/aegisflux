Implement app/writers/neo4j.py:
- connect() returns a neo4j.Driver.
- For "connect" events, expect JSON shape: args.dst_ip, args.dst_port, and source host_id.
- Upsert:
  MERGE (a:Host {host_id:$src})
  MERGE (b:Host {host_id:$dst})
  MERGE (a)-[r:COMMUNICATES]->(b)
  ON CREATE SET r.count_1h = 1, r.last_seen = timestamp()
  ON MATCH SET  r.count_1h = coalesce(r.count_1h,0) + 1, r.last_seen = timestamp();
- The $dst host_id can be derived from IP if it's an internal known host; else create/merge as NetworkEndpoint (optional for MVP). For MVP, assume host_id for both src/dst if available; if not, set dst host_id="ip:<dst_ip>".
- Provide a function upsert_comm_edge(src_host_id, dst_host_id).
- Unit test the Cypher string & parameters via a fake session/transaction object (no external DB).
