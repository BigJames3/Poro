"""The {data, error, meta} envelope and error rendering shared with the other services."""

import logging
import re
from typing import Any

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse, Response
from starlette.exceptions import HTTPException as StarletteHTTPException
from starlette.middleware.base import BaseHTTPMiddleware, RequestResponseEndpoint

from moderation.cases import DomainError
from moderation.ids import uuid7

HEADER_REQUEST_ID = "X-Request-ID"
_REQUEST_ID = re.compile(r"^[A-Za-z0-9._-]{1,128}$")
log = logging.getLogger(__name__)

_CODES = {
    400: ("invalid_request", "invalid request"),
    401: ("unauthorized", "unauthorized"),
    403: ("forbidden", "forbidden"),
    404: ("not_found", "not found"),
    405: ("method_not_allowed", "method not allowed"),
    413: ("payload_too_large", "payload too large"),
    415: ("unsupported_media_type", "unsupported media type"),
    429: ("rate_limited", "too many requests"),
    503: ("unavailable", "service unavailable"),
}


def request_id(request: Request) -> str:
    return str(getattr(request.state, "request_id", ""))


def data(request: Request, payload: Any, status: int = 200) -> JSONResponse:
    return JSONResponse(
        {"data": payload, "error": None, "meta": {"request_id": request_id(request)}}, status_code=status
    )


def error(
    request: Request, status: int, code: str, message: str, headers: dict[str, str] | None = None
) -> JSONResponse:
    return JSONResponse(
        {
            "data": None,
            "error": {"code": code, "message": message},
            "meta": {"request_id": request_id(request)},
        },
        status_code=status,
        headers=headers,
    )


class RequestIdMiddleware(BaseHTTPMiddleware):
    """Reuses a well-formed incoming X-Request-ID or generates a UUIDv7, and echoes it."""

    async def dispatch(self, request: Request, call_next: RequestResponseEndpoint) -> Response:
        incoming = request.headers.get(HEADER_REQUEST_ID, "")
        request.state.request_id = incoming if _REQUEST_ID.match(incoming) else str(uuid7())
        response = await call_next(request)
        response.headers[HEADER_REQUEST_ID] = request.state.request_id
        return response


def _first_validation_message(err: RequestValidationError) -> tuple[str, str]:
    for item in err.errors():
        if item.get("type") == "json_invalid":
            return "invalid_json", "invalid json"
        loc = ".".join(str(part) for part in item.get("loc", ()) if part not in ("body", "query", "path"))
        message = str(item.get("msg", "invalid request"))
        return "invalid_request", f"{loc}: {message}" if loc else message
    return "invalid_request", "invalid request"


def install_error_handlers(app: FastAPI) -> None:
    @app.exception_handler(DomainError)
    async def domain(request: Request, exc: DomainError) -> JSONResponse:
        retry_after = getattr(exc, "retry_after", None)
        headers = None if retry_after is None else {"Retry-After": str(retry_after)}
        return error(request, exc.status, exc.code, exc.message, headers)

    @app.exception_handler(RequestValidationError)
    async def validation(request: Request, exc: RequestValidationError) -> JSONResponse:
        code, message = _first_validation_message(exc)
        return error(request, 400, code, message)

    @app.exception_handler(StarletteHTTPException)
    async def http(request: Request, exc: StarletteHTTPException) -> JSONResponse:
        code, message = _CODES.get(exc.status_code, ("error", "error"))
        return error(request, exc.status_code, code, message, getattr(exc, "headers", None))

    @app.exception_handler(Exception)
    async def internal(request: Request, exc: Exception) -> JSONResponse:
        log.exception("request failed", extra={"request_id": request_id(request), "path": request.url.path})
        return error(request, 500, "internal_error", "internal server error")
