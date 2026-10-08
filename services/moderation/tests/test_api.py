"""HTTP API against a real Postgres, with tokens signed like the auth service."""

from typing import Any

import httpx
from fastapi import FastAPI

from moderation.ids import uuid7
from tests.conftest import Keys

MOD = ["PERSONAL", "MODERATOR"]


def bearer(token: str) -> dict[str, str]:
    return {"Authorization": f"Bearer {token}"}


async def test_probes_and_metrics(http: httpx.AsyncClient) -> None:
    for path in ("/health", "/health/ready", "/api/v1/health"):
        res = await http.get(path)
        assert res.status_code == 200
        assert res.json() == {
            "status": "ok",
            "service": "moderation",
            "version": "1.0.0",
            "checks": {"postgres": "up"},
        }
    assert (await http.get("/health/live")).json()["status"] == "ok"
    metrics = await http.get("/metrics")
    assert "http_requests_total" in metrics.text
    res = await http.get("/nowhere", headers={"X-Request-ID": "abc-123"})
    assert res.status_code == 404
    assert res.headers["X-Request-ID"] == "abc-123"
    assert res.json() == {
        "data": None,
        "error": {"code": "not_found", "message": "not found"},
        "meta": {"request_id": "abc-123"},
    }


async def test_requires_a_valid_token(http: httpx.AsyncClient, keys: Keys) -> None:
    body = {"target_type": "video", "target_id": str(uuid7()), "reason": "spam"}
    for headers in (
        {},
        {"Authorization": "Basic abc"},
        bearer("not-a-jwt"),
        bearer(keys.token(aud="other")),
        bearer(keys.token(sub="not-a-uuid")),
        bearer(keys.token(roles="ADMIN")),
        bearer(keys.token(kid=None)),
        bearer(keys.token(kid="unknown")),
        bearer(keys.token(sid=None)),
    ):
        res = await http.post("/api/v1/moderation/reports", json=body, headers=headers)
        assert res.status_code == 401, headers
        assert res.json()["error"]["code"] == "unauthorized"


async def test_report_flow(http: httpx.AsyncClient, keys: Keys) -> None:
    target = str(uuid7())
    body = {"target_type": "comment", "target_id": target, "reason": "harassment", "comment": "  insulte  "}
    first = await http.post("/api/v1/moderation/reports", json=body, headers=bearer(keys.token()))
    assert first.status_code == 201
    case_id = first.json()["data"]["case_id"]
    reporter = str(uuid7())
    again = [
        await http.post("/api/v1/moderation/reports", json=body, headers=bearer(keys.token(sub=reporter)))
        for _ in range(2)
    ]
    assert [r.status_code for r in again] == [201, 200]
    assert again[0].json()["data"]["report_id"] == again[1].json()["data"]["report_id"]
    assert again[1].json()["data"]["created"] is False

    for bad, code in (
        ({**body, "target_type": "shop"}, "invalid_request"),
        ({**body, "reason": "boring"}, "invalid_request"),
        ({**body, "comment": "x" * 501}, "invalid_request"),
        ({**body, "extra": 1}, "invalid_request"),
    ):
        res = await http.post("/api/v1/moderation/reports", json=bad, headers=bearer(keys.token()))
        assert res.status_code == 400
        assert res.json()["error"]["code"] == code
    res = await http.post(
        "/api/v1/moderation/reports",
        content=b"{nope",
        headers={**bearer(keys.token()), "content-type": "application/json"},
    )
    assert res.json()["error"]["code"] == "invalid_json"

    me = str(uuid7())
    res = await http.post(
        "/api/v1/moderation/reports",
        json={"target_type": "user", "target_id": me, "reason": "spam"},
        headers=bearer(keys.token(sub=me)),
    )
    assert res.status_code == 422
    assert res.json()["error"]["code"] == "cannot_report_self"

    # Only moderators see the queue.
    res = await http.get(f"/api/v1/moderation/cases/{case_id}", headers=bearer(keys.token()))
    assert res.status_code == 403
    case = await http.get(f"/api/v1/moderation/cases/{case_id}", headers=bearer(keys.token(roles=MOD)))
    assert case.status_code == 200
    view: dict[str, Any] = case.json()["data"]
    assert view["reports_count"] == 2
    assert view["content"] is None
    assert {r["comment"] for r in view["reports"]} == {"insulte"}


async def test_reports_are_rate_limited(http: httpx.AsyncClient, keys: Keys) -> None:
    token = keys.token()
    codes = []
    for _ in range(11):
        res = await http.post(
            "/api/v1/moderation/reports",
            json={"target_type": "video", "target_id": str(uuid7()), "reason": "spam"},
            headers=bearer(token),
        )
        codes.append(res.status_code)
    assert codes[:10] == [201] * 10
    assert codes[10] == 429
    assert int(res.headers["Retry-After"]) > 0


async def test_queue_and_decisions(http: httpx.AsyncClient, keys: Keys) -> None:
    mod = bearer(keys.token(roles=["ADMIN"]))
    target = str(uuid7())
    created = await http.post(
        "/api/v1/moderation/reports",
        json={"target_type": "user", "target_id": target, "reason": "fraud"},
        headers=bearer(keys.token()),
    )
    case_id = created.json()["data"]["case_id"]

    listing = await http.get(
        "/api/v1/moderation/cases?target_type=user&priority=normal&limit=50", headers=mod
    )
    assert listing.status_code == 200
    assert case_id in [c["id"] for c in listing.json()["data"]["items"]]
    for query in ("status=gone", "limit=0", "limit=51", "cursor=bad", "priority=urgent"):
        res = await http.get(f"/api/v1/moderation/cases?{query}", headers=mod)
        assert res.status_code == 400, query

    remove = await http.post(
        f"/api/v1/moderation/cases/{case_id}/decision",
        json={"action": "remove", "reason": "fraud"},
        headers=mod,
    )
    assert remove.status_code == 422
    assert remove.json()["error"]["code"] == "action_not_allowed"
    dismiss = await http.post(
        f"/api/v1/moderation/cases/{case_id}/decision", json={"action": "dismiss", "note": "ok"}, headers=mod
    )
    assert dismiss.status_code == 200
    assert dismiss.json()["data"]["status"] == "dismissed"
    assert dismiss.json()["data"]["decisions"][0]["decided_by"] == "moderator"
    again = await http.post(
        f"/api/v1/moderation/cases/{case_id}/decision", json={"action": "dismiss"}, headers=mod
    )
    assert again.status_code == 409

    for bad in ("not-a-uuid", str(uuid7())):
        res = await http.get(f"/api/v1/moderation/cases/{bad}", headers=mod)
        assert res.status_code == 404
        assert res.json()["error"]["code"] == "case_not_found"
    res = await http.post(f"/api/v1/moderation/cases/{case_id}/decision", json={"action": "ban"}, headers=mod)
    assert res.status_code == 400


async def test_jwks_outage(app: FastAPI, http: httpx.AsyncClient, keys: Keys) -> None:
    """Known keys keep working while auth is down; unknown ones answer 503."""
    keys.status = 500
    try:
        app.state.verifier._keys._last_attempt = float("-inf")
        app.state.verifier._keys._fetched_at = 0.0
        known = await http.get("/api/v1/moderation/cases", headers=bearer(keys.token(roles=MOD)))
        assert known.status_code == 200
        unknown = await http.get(
            "/api/v1/moderation/cases", headers=bearer(keys.token(roles=MOD, kid="rotated"))
        )
        assert unknown.status_code == 503
        assert unknown.json()["error"]["code"] == "unavailable"
    finally:
        keys.status = 200
        app.state.verifier._keys._last_attempt = float("-inf")
