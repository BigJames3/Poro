"""/api/v1/moderation: reports from every account, the queue for moderators."""

import uuid
from typing import Annotated, Literal

from fastapi import APIRouter, Query, Request
from fastapi.responses import JSONResponse
from prometheus_client import Counter

from moderation import cases
from moderation.api.schemas import DecisionRequest, ReportRequest
from moderation.http.auth import CurrentUser, Moderator
from moderation.http.errors import data
from moderation.http.ratelimit import enforce

REPORTS_PER_HOUR = 10
REQUESTS_PER_MINUTE = 120
DEFAULT_PAGE, MAX_PAGE = 20, 50

DECISIONS = Counter("moderation_decisions_total", "Moderator decisions by action.", ["service", "action"])
REPORTS = Counter("moderation_reports_total", "Reports filed by target type.", ["service", "target_type"])

router = APIRouter(prefix="/api/v1/moderation")


async def _general_limit(request: Request, user_id: uuid.UUID) -> None:
    await enforce(request.app.state.limiter, f"all:{user_id}", REQUESTS_PER_MINUTE, 60)


@router.post("/reports")
async def create_report(request: Request, body: ReportRequest, user: CurrentUser) -> JSONResponse:
    await _general_limit(request, user.user_id)
    await enforce(request.app.state.limiter, f"reports:{user.user_id}", REPORTS_PER_HOUR, 3600)
    async with request.app.state.sessions.begin() as session:
        result = await cases.report(
            session,
            user.user_id,
            body.target_type,
            body.target_id,
            body.reason,
            body.comment.strip() if body.comment and body.comment.strip() else None,
            request.app.state.config.report_threshold,
        )
    if result.created:
        REPORTS.labels("moderation", body.target_type).inc()
    return data(
        request,
        {"report_id": str(result.report_id), "case_id": str(result.case_id), "created": result.created},
        201 if result.created else 200,
    )


@router.get("/cases")
async def list_cases(
    request: Request,
    user: Moderator,
    status: Annotated[Literal["open", "removed", "dismissed", "restored"], Query()] = "open",
    target_type: Annotated[Literal["video", "comment", "user"] | None, Query()] = None,
    priority: Annotated[Literal["normal", "high"] | None, Query()] = None,
    cursor: Annotated[str | None, Query(max_length=200)] = None,
    limit: Annotated[int, Query(ge=1, le=MAX_PAGE)] = DEFAULT_PAGE,
) -> JSONResponse:
    await _general_limit(request, user.user_id)
    level = None if priority is None else (cases.HIGH if priority == "high" else cases.NORMAL)
    async with request.app.state.sessions() as session:
        page = await cases.list_cases(session, status, target_type, level, cursor, limit)
    return data(request, page)


@router.get("/cases/{case_id}")
async def get_case(request: Request, case_id: str, user: Moderator) -> JSONResponse:
    await _general_limit(request, user.user_id)
    async with request.app.state.sessions() as session:
        return data(request, await cases.get_case(session, _case_id(case_id)))


@router.post("/cases/{case_id}/decision")
async def decide(request: Request, case_id: str, body: DecisionRequest, user: Moderator) -> JSONResponse:
    await _general_limit(request, user.user_id)
    async with request.app.state.sessions.begin() as session:
        view = await cases.decide(
            session, _case_id(case_id), user.user_id, body.action, body.reason, body.note
        )
    DECISIONS.labels("moderation", body.action).inc()
    return data(request, view)


def _case_id(raw: str) -> uuid.UUID:
    try:
        return uuid.UUID(raw)
    except ValueError as err:
        raise cases.DomainError(404, "case_not_found", "case not found") from err
