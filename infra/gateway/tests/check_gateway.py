"""Contract test of the PORO gateway, run against Traefik and whoami stubs.

Every path of docs/api/<service>.openapi.yaml under /api/v1/ must reach
<service>; health, metrics and JWKS paths must not be reachable. The script
then checks the gateway's own behaviour: body limit, CORS, compression,
security headers, forwarded headers and rate limiting.

Each stub backend is traefik/whoami with --hostname set to its service name,
so the echoed "Hostname:" line names the service that answered.

Usage: python3 check_gateway.py <gateway base URL> <docs/api directory>
Only the standard library is used.
"""

import concurrent.futures
import gzip
import http.client
import re
import sys
import urllib.parse
from pathlib import Path

SAMPLE_ID = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b"
ALLOWED_ORIGIN = "https://app.poro.test"


def openapi_routes(api_dir):
    """Yields (method, path, service, public) for every documented operation."""
    for spec in sorted(Path(api_dir).glob("*.openapi.yaml")):
        service = spec.name.split(".")[0]
        path = None
        for line in spec.read_text(encoding="utf-8").splitlines():
            match = re.match(r"^  (/[^:]+):\s*$", line)
            if match:
                path = match.group(1)
                continue
            match = re.match(r"^    (get|post|put|patch|delete):", line)
            if match and path:
                public = path.startswith("/api/v1/") and path != "/api/v1/health"
                yield match.group(1).upper(), path, service, public


class Gateway:
    def __init__(self, base_url):
        parsed = urllib.parse.urlsplit(base_url)
        self.host, self.port = parsed.hostname, parsed.port or 80

    def request(self, method, path, body=None, headers=None):
        conn = http.client.HTTPConnection(self.host, self.port, timeout=10)
        try:
            conn.request(method, path, body=body, headers=headers or {})
            resp = conn.getresponse()
            return resp.status, {k.lower(): v for k, v in resp.getheaders()}, resp.read()
        finally:
            conn.close()


def answered_by(body):
    match = re.search(rb"^Hostname: (\S+)", body, re.MULTILINE)
    return match.group(1).decode() if match else None


def check_routes(gw, api_dir, failures):
    count = 0
    for method, path, service, public in openapi_routes(api_dir):
        concrete = re.sub(r"\{[^}]+\}", SAMPLE_ID, path)
        body = b"{}" if method in ("POST", "PUT", "PATCH") else None
        status, _, payload = gw.request(method, concrete, body=body,
                                        headers={"Content-Type": "application/json"})
        got = answered_by(payload) if status == 200 else None
        if public and got != service:
            failures.append(f"{method} {path}: expected {service}, got status {status} from {got}")
        if not public and status != 404:
            failures.append(f"{method} {path} ({service}) must not be public, got {status}")
        count += 1
    # Prefixes are whole segments, and unknown paths reach nobody.
    for path in ("/api/v1/authx/otp", "/api/v1/unknown", "/metrics", "/", "/api/v1/videosx"):
        status, _, _ = gw.request("GET", path)
        if status != 404:
            failures.append(f"GET {path}: expected 404, got {status}")
    return count


def check_behaviour(gw, failures):
    status, _, _ = gw.request("GET", "/health")
    if status != 200:
        failures.append(f"gateway /health: {status}")

    big = b"x" * (1024 * 1024 + 1)
    status, _, _ = gw.request("POST", "/api/v1/auth/email/login", body=big)
    if status != 413:
        failures.append(f"body over 1 MiB: expected 413, got {status}")

    preflight = {"Origin": ALLOWED_ORIGIN, "Access-Control-Request-Method": "POST",
                 "Access-Control-Request-Headers": "authorization,content-type"}
    status, headers, _ = gw.request("OPTIONS", "/api/v1/orders", headers=preflight)
    if headers.get("access-control-allow-origin") != ALLOWED_ORIGIN:
        failures.append(f"CORS preflight from an allowed origin: {status} {headers}")
    denied = dict(preflight, Origin="https://evil.test")
    _, headers, _ = gw.request("OPTIONS", "/api/v1/orders", headers=denied)
    if "access-control-allow-origin" in headers:
        failures.append("CORS allowed an unknown origin")

    padding = "p" * 2000
    status, headers, payload = gw.request("GET", "/api/v1/feed/trending",
                                          headers={"Accept-Encoding": "gzip", "X-Padding": padding})
    if headers.get("content-encoding") != "gzip" or padding.encode() not in gzip.decompress(payload):
        failures.append(f"large responses must be gzip-compressed: {headers.get('content-encoding')}")

    _, headers, payload = gw.request("GET", "/api/v1/search", headers={"X-Forwarded-For": "6.6.6.6"})
    if headers.get("x-content-type-options") != "nosniff" or headers.get("x-frame-options") != "DENY":
        failures.append(f"security headers missing: {headers}")
    forwarded = re.search(rb"^X-Forwarded-For: (.*)$", payload, re.MULTILINE)
    if forwarded is None or b"6.6.6.6" in forwarded.group(1):
        failures.append(f"a client-supplied X-Forwarded-For reached the service: {forwarded}")


def check_rate_limit(gw, failures):
    def hit(_):
        return gw.request("GET", "/api/v1/search/suggest")[0]

    with concurrent.futures.ThreadPoolExecutor(max_workers=50) as pool:
        statuses = list(pool.map(hit, range(600)))
    if 429 not in statuses:
        failures.append("600 requests in a burst from one IP were never limited")


def main():
    gw = Gateway(sys.argv[1])
    failures = []
    count = check_routes(gw, sys.argv[2], failures)
    check_behaviour(gw, failures)
    check_rate_limit(gw, failures)
    if failures:
        print(f"{len(failures)} gateway check(s) failed:")
        for failure in failures:
            print(f"  - {failure}")
        sys.exit(1)
    print(f"gateway: {count} documented routes and the gateway behaviour check out")


if __name__ == "__main__":
    main()
