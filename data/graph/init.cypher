CREATE CONSTRAINT host_id_unique IF NOT EXISTS
FOR (h:Host) REQUIRE h.host_id IS UNIQUE;

CREATE INDEX comm_last_seen IF NOT EXISTS
FOR ()-[r:COMMUNICATES]-() ON (r.last_seen);
