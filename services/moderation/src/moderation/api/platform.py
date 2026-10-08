"""Probes and metrics. Liveness checks nothing, so a database outage does not
restart every pod; readiness checks Postgres and Redis. Kafka is left out:
reports and the queue keep working while the broker is down."""

import asyncio
from collections.abc import Awaitable, Callable

from fastapi import APIRouter, Request
from fastapi.responses import JSONResponse, Response
from prometheus_client import CONTENT_TYPE_LATEST, generate_latest
from sqlalchemy import text

from moderation import SERVICE_NAME, SERVICE_VERSION

router = APIRouter()
CHECK_TIMEOUT_S = 2.0


async def _postgres(request: Request) -> None:
    async with request.app.state.engine.connect() as conn:
        await conn.execute(text("SELECT 1"))


async def _redis(request: Request) -> None:
    await request.app.state.redis.ping()


@router.get("/health/live")
async def live() -> JSONResponse:
    return JSONResponse({"status": "ok", "service": SERVICE_NAME, "version": SERVICE_VERSION})


@router.get("/health")
@router.get("/health/ready")
@router.get("/api/v1/health")
async def ready(request: Request) -> JSONResponse:
    checks: dict[str, Callable[[Request], Awaitable[None]]] = {"postgres": _postgres}
    if request.app.state.redis is not None:
        checks["redis"] = _redis
    results = await asyncio.gather(
        *(asyncio.wait_for(check(request), CHECK_TIMEOUT_S) for check in checks.values()),
        return_exceptions=True,
    )
    states = {
        name: "down" if isinstance(r, BaseException) else "up"
        for name, r in zip(checks, results, strict=True)
    }
    ok = all(state == "up" for state in states.values())
    return JSONResponse(
        {
            "status": "ok" if ok else "degraded",
            "service": SERVICE_NAME,
            "version": SERVICE_VERSION,
            "checks": states,
        },
        status_code=200 if ok else 503,
    )


@router.get("/metrics")
async def metrics() -> Response:
    """Must not be exposed by the public gateway."""
    return Response(generate_latest(), media_type=CONTENT_TYPE_LATEST)
