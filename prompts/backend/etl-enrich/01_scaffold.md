Create a Python 3.11 project in backend/etl-enrich:

- Structure:
  backend/etl-enrich/app/{consumer.py,enrich.py,publish.py,config.py}
  backend/etl-enrich/app/writers/{timescale.py,neo4j.py}
  backend/etl-enrich/tests/{test_enrich.py,test_writers.py,conftest.py}
  backend/etl-enrich/requirements.txt
  backend/etl-enrich/Dockerfile
  backend/etl-enrich/README.md

- requirements.txt: nats-py, psycopg[binary], neo4j, pydantic, orjson, python-dotenv, tenacity, pytest.

- config.py: load env (NATS_URL, PG_*, NEO4J_*, AF_ENV, AF_FAKE_RDNS).
- consumer.py: NATS subscribe to "events.raw" (queue group "etl"), process messages with backpressure.
- publish.py: publish enriched JSON to "events.enriched".
- enrich.py: add {"env": AF_ENV, "rdns": fake "host-<last_octet>.local" if AF_FAKE_RDNS=true}.
- README: run instructions & env list.
