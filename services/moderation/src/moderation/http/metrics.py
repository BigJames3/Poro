"""HTTP metrics with the same names and labels as shared-go/metrics."""

import time

from fastapi import Request, Response
from prometheus_client import Counter, Histogram
from starlette.middleware.base import BaseHTTPMiddleware, RequestResponseEndpoint
from starlette.routing import Match

from moderation import SERVICE_NAME

REQUESTS = Counter(
    "http_requests_total",
    "HTTP requests by method, route template and status.",
    ["service", "method", "route", "status"],
)
DURATION = Histogram(
    "http_request_duration_seconds",
    "HTTP request latency by method and route template.",
    ["service", "method", "route"],
    buckets=(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
)


def _route(request: Request) -> str:
    """The route template, never the raw path, to keep cardinality bounded."""
    for route in request.app.router.routes:
        match, _ = route.matches(request.scope)
        if match is Match.FULL:
            return str(getattr(route, "path", "unmatched"))
    return "unmatched"


class MetricsMiddleware(BaseHTTPMiddleware):
    async def dispatch(self, request: Request, call_next: RequestResponseEndpoint) -> Response:
        start = time.perf_counter()
        response = await call_next(request)
        route = _route(request)
        REQUESTS.labels(SERVICE_NAME, request.method, route, str(response.status_code)).inc()
        DURATION.labels(SERVICE_NAME, request.method, route).observe(time.perf_counter() - start)
        return response
