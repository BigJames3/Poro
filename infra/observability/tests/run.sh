#!/usr/bin/env bash
# Observability stack checks (PORO-009): alert rules unit tests, config
# validation, then Prometheus, Alertmanager and Grafana started together on an
# isolated network with the Compose images and files.
set -euo pipefail

root=$(cd "$(dirname "$0")/../../.." && pwd)
obs="$root/infra/observability"
net="poro-obs-test-$$"
names=("poro-obs-prom-$$" "poro-obs-am-$$" "poro-obs-graf-$$")
cleanup() {
  docker rm -f "${names[@]}" >/dev/null 2>&1 || true
  docker network rm "$net" >/dev/null 2>&1 || true
}
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "ok    $*"; }
image() { awk -v svc="  $1:" '$0 == svc { found = 1 } found && /image:/ { print $2; exit }' "$root/docker-compose.yml"; }

prom=$(image prometheus)
am=$(image alertmanager)
graf=$(image grafana)
for img in "$prom" "$am" "$graf"; do
  [ -n "$img" ] || fail "observability images not found in docker-compose.yml"
done

docker run --rm -v "$obs/rules:/rules:ro" -w /rules --entrypoint promtool "$prom" test rules poro.rules.test.yml
pass "alert rules unit tests"
docker run --rm -v "$obs:/o:ro" --entrypoint promtool "$prom" check config --syntax-only /o/prometheus.yml >/dev/null
docker run --rm -v "$obs/alertmanager.yml:/a.yml:ro" --entrypoint amtool "$am" check-config /a.yml >/dev/null
python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$obs/grafana/dashboards/poro-overview.json"
pass "prometheus, alertmanager and dashboard files are valid"

docker network create "$net" >/dev/null
docker run -d --name "${names[1]}" --network "$net" --network-alias alertmanager \
  -v "$obs/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro" "$am" \
  --config.file=/etc/alertmanager/alertmanager.yml >/dev/null
docker run -d --name "${names[0]}" --network "$net" --network-alias prometheus -p 127.0.0.1::9090 \
  -v "$obs/prometheus.yml:/etc/prometheus/prometheus.yml:ro" -v "$obs/rules:/etc/prometheus/rules:ro" "$prom" \
  --config.file=/etc/prometheus/prometheus.yml >/dev/null
docker run -d --name "${names[2]}" --network "$net" -p 127.0.0.1::3000 \
  -e GF_AUTH_ANONYMOUS_ENABLED=true -e GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer \
  -e GF_AUTH_DISABLE_LOGIN_FORM=true -e GF_AUTH_BASIC_ENABLED=false \
  -v "$obs/grafana/provisioning:/etc/grafana/provisioning:ro" \
  -v "$obs/grafana/dashboards:/var/lib/grafana/dashboards:ro" "$graf" >/dev/null
pport=$(docker port "${names[0]}" 9090/tcp | head -1 | sed 's/.*://')
gport=$(docker port "${names[2]}" 3000/tcp | head -1 | sed 's/.*://')

wait_for() { # url, jq-free python predicate on the JSON body
  for _ in $(seq 1 90); do
    if body=$(curl -sf "$1" 2>/dev/null) && python3 -c "$2" "$body" 2>/dev/null; then return 0; fi
    sleep 1
  done
  return 1
}

wait_for "http://127.0.0.1:$pport/api/v1/rules" \
  'import json,sys; d=json.loads(sys.argv[1]); assert sum(len(g["rules"]) for g in d["data"]["groups"]) == 9' ||
  fail "prometheus did not load the 9 alert rules"
pass "prometheus loaded the 9 alert rules"

wait_for "http://127.0.0.1:$pport/api/v1/alertmanagers" \
  'import json,sys; d=json.loads(sys.argv[1]); assert len(d["data"]["activeAlertmanagers"]) == 1' ||
  fail "prometheus does not see alertmanager"
pass "prometheus sends alerts to alertmanager"

wait_for "http://127.0.0.1:$pport/api/v1/targets" \
  'import json,sys; d=json.loads(sys.argv[1]); assert len(d["data"]["activeTargets"]) == 19' ||
  fail "prometheus does not scrape the 19 expected targets"
pass "19 scrape targets (10 APIs, 7 outbox relays, gateway, redpanda)"

wait_for "http://127.0.0.1:$gport/api/search?query=PORO" \
  'import json,sys; d=json.loads(sys.argv[1]); assert any(x["uid"] == "poro-overview" for x in d)' ||
  fail "grafana did not provision the PORO dashboard"
pass "grafana serves the provisioned dashboard read-only"

code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:$gport/api/dashboards/db" \
  -H 'Content-Type: application/json' -d '{"dashboard":{"title":"x"}}')
[ "$code" = 401 ] || [ "$code" = 403 ] || fail "anonymous user could write to grafana (HTTP $code)"
pass "anonymous grafana user cannot write (HTTP $code)"

echo "observability stack: all checks passed"
