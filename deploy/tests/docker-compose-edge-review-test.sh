#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

if ! command -v docker >/dev/null 2>&1; then
  printf 'docker compose edge review test skipped: docker is unavailable\n'
  exit 0
fi

tmp=$(mktemp "${TMPDIR:-/tmp}/sub2api-compose-edge.XXXXXX")
env_file=$(mktemp "${TMPDIR:-/tmp}/sub2api-compose-env.XXXXXX")
cleanup() { rm -f "$tmp"; }
cleanup() { rm -f "$tmp" "$env_file"; }
trap cleanup EXIT HUP INT TERM

for compose_file in \
  deploy/docker-compose.yml \
  deploy/docker-compose.local.yml \
  deploy/docker-compose.dev.yml \
  deploy/docker-compose.standalone.yml
do
  LOBEHUB_SSO_SHARED_SECRET= \
    LOBEHUB_SSO_ENABLED=false \
    POSTGRES_PASSWORD=test \
    DATABASE_PASSWORD=test \
    DATABASE_HOST=postgres \
    DATABASE_USER=sub2api \
    DATABASE_DBNAME=sub2api \
    REDIS_HOST=redis \
    REDIS_PORT=6379 \
    docker compose --env-file "$env_file" -f "$compose_file" config --format json >"$tmp"
  python3 - "$tmp" "$compose_file" <<'PY'
import json
import sys

config = json.load(open(sys.argv[1]))
name = sys.argv[2]
services = config["services"]
app = services["sub2api"]
env = {}
raw_environment = app.get("environment", {})
if isinstance(raw_environment, dict):
    env = raw_environment
else:
    for item in raw_environment:
        key, value = item.split("=", 1)
        env[key] = value

assert env.get("LOBEHUB_SSO_ENABLED") == "false", name
assert env.get("LOBEHUB_SSO_SHARED_SECRET", "") == "", name
assert env.get("GATEWAY_OPENAI_COMPACT_MODEL") == "gpt-5.5", name
for key in (
    "GATEWAY_OPENAI_CODEX_TICKET_ENABLED",
    "GATEWAY_OPENAI_CODEX_TICKET_TARGET_LENGTH",
    "GATEWAY_OPENAI_CODEX_TICKET_TTL_SECONDS",
    "GATEWAY_OPENAI_CODEX_TICKET_REFRESH_BEFORE_SECONDS",
    "GATEWAY_OPENAI_CODEX_TICKET_HARVEST_PROXY_URL",
    "GATEWAY_OPENAI_CODEX_TICKET_HARVEST_PROBE_INTERVAL_SECONDS",
    "GATEWAY_OPENAI_CODEX_TICKET_HARVEST_ATTEMPT_TIMEOUT_SECONDS",
    "GATEWAY_OPENAI_CODEX_TICKET_FAIL_CLOSED",
    "GATEWAY_OPENAI_CODEX_TICKET_MODELS",
):
    assert key in env, (name, key)

assert "intel-check-motion" in services, name
assert app["depends_on"]["intel-check-motion"]["condition"] == "service_healthy", name
assert "intel-check-motion-network" in app["networks"], name
assert config["networks"]["intel-check-motion-network"]["internal"] is True, name
PY
done

printf 'docker compose edge review test passed\n'
