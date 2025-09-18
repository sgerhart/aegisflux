#!/usr/bin/env bash
set -euo pipefail
VAULT_ADDR="${VAULT_ADDR:-http://localhost:8200}"
VAULT_TOKEN="${VAULT_TOKEN:-root}"

echo "[*] Enabling KV v2 at secret/"
vault login -no-print "$VAULT_TOKEN" >/dev/null
vault secrets enable -path=secret -version=2 kv || true

echo "[*] Writing example secrets"
vault kv put secret/llm/openai API_KEY="REPLACE_ME"
vault kv put secret/nats creds="$(printf 'nats://nats:4222')"
vault kv put secret/neo4j user="neo4j" pass="password"
vault kv put secret/registry signing_key="dev-not-real"

echo "[*] Creating policies"
cat > /tmp/p-decision.hcl <<'POL'
path "secret/data/llm/*"     { capabilities = ["read"] }
path "secret/data/registry/*" { capabilities = ["read"] }
POL
vault policy write decision /tmp/p-decision.hcl

cat > /tmp/p-correlator.hcl <<'POL'
path "secret/data/nats"  { capabilities = ["read"] }
path "secret/data/neo4j" { capabilities = ["read"] }
POL
vault policy write correlator /tmp/p-correlator.hcl
rm -f /tmp/p-*.hcl

echo "[*] Creating dev tokens (use AppRole/JWT in prod)"
DECISION_TOKEN=$(vault token create -policy=decision -format=json | jq -r .auth.client_token)
CORRELATOR_TOKEN=$(vault token create -policy=correlator -format=json | jq -r .auth.client_token)
echo "DECISION_TOKEN=$DECISION_TOKEN"
echo "CORRELATOR_TOKEN=$CORRELATOR_TOKEN"
