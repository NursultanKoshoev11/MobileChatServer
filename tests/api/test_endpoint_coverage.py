"""
Full API endpoint coverage tests for MobileChatServer.

This suite intentionally has two layers:
1. route inventory coverage: every chi route declared in internal/httpapi/server.go
   must be present in tests/api/endpoints_manifest.json;
2. executable HTTP checks: public routes, auth flow, no-auth protection for every
   protected route, and fixture-based business calls for resource-specific routes.

Run:
  python -m pip install -r tests/api/requirements.txt
  API_BASE_URL=http://127.0.0.1:8080 \
  API_TEST_PHONE=+996700000001 \
  API_TEST_CODE=111111 \
  API_STRICT_FULL=1 \
  python -m pytest tests/api -v

Strict full mode requires fixture IDs for resource-specific endpoints:
  API_TEST_GROUP_ID
  API_TEST_TARGET_USER_ID
  API_TEST_INVITE_ID
  API_TEST_REQUEST_ID
  API_TEST_COMMENT_ID
  API_TEST_MODERATION_ITEM_ID
  API_TEST_ADMIN_GROUP_CREATION_REQUEST_ID
"""

from __future__ import annotations

import json
import os
import re
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import pytest
import requests


ROOT = Path(__file__).resolve().parents[2]
SERVER_GO = ROOT / "internal" / "httpapi" / "server.go"
MANIFEST = Path(__file__).with_name("endpoints_manifest.json")


@dataclass(frozen=True)
class Endpoint:
    method: str
    path: str
    auth: bool
    area: str


def _load_manifest() -> list[Endpoint]:
    raw = json.loads(MANIFEST.read_text(encoding="utf-8"))
    return [Endpoint(**item) for item in raw["endpoints"]]


def _discover_routes_from_server_go() -> set[tuple[str, str]]:
    source = SERVER_GO.read_text(encoding="utf-8")
    found: set[tuple[str, str]] = set()

    # Routes are declared as r.Get("..."), r.Post("..."), r.Delete("...").
    # The only nested prefix in the current router is /api/auth.
    for method, path in re.findall(r"r\.(Get|Post|Delete)\(\"([^\"]+)\"", source):
        full_path = path
        if not full_path.startswith("/api/"):
            full_path = "/api/auth" + full_path
        found.add((method.upper(), full_path))

    return found


@pytest.fixture(scope="session")
def base_url() -> str:
    return os.getenv("API_BASE_URL", "http://127.0.0.1:8080").rstrip("/")


@pytest.fixture(scope="session")
def http_session() -> requests.Session:
    session = requests.Session()
    session.headers.update({"Accept": "application/json"})
    return session


def _json_or_text(response: requests.Response) -> Any:
    try:
        return response.json()
    except ValueError:
        return response.text


def _request(
    session: requests.Session,
    base_url: str,
    method: str,
    path: str,
    *,
    token: str | None = None,
    body: dict[str, Any] | None = None,
    expected: set[int] | None = None,
) -> requests.Response:
    headers = {"Content-Type": "application/json; charset=utf-8"}
    if token:
        headers["Authorization"] = f"Bearer {token}"

    response = session.request(
        method,
        f"{base_url}{path}",
        headers=headers,
        json=body if body is not None else {},
        timeout=20,
    )
    if expected is not None and response.status_code not in expected:
        pytest.fail(
            f"{method} {path} returned {response.status_code}, expected one of "
            f"{sorted(expected)}. Body: {_json_or_text(response)!r}"
        )
    return response


def _body_for(endpoint: Endpoint) -> dict[str, Any]:
    path = endpoint.path

    if path == "/api/auth/request-code":
        return {"mobile": os.getenv("API_TEST_PHONE", "+996700000001")}
    if path == "/api/auth/verify-code":
        return {
            "mobile": os.getenv("API_TEST_PHONE", "+996700000001"),
            "code": os.getenv("API_TEST_CODE", "111111"),
            "display_name": os.getenv("API_TEST_DISPLAY_NAME", "API Test User"),
        }
    if path in {"/api/auth/refresh", "/api/auth/logout"}:
        return {"refresh_token": os.getenv("API_TEST_REFRESH_TOKEN", "invalid-refresh-token")}
    if path == "/api/push/register" or path == "/api/push/token":
        return {"token": "qa-test-token", "platform": "android"}
    if path == "/api/groups":
        return {
            "title": f"QA Test Group {int(time.time())}",
            "description": "Created by endpoint coverage test.",
            "visibility": "public",
        }
    if path == "/api/groups/join-by-code":
        return {"invite_code": os.getenv("API_TEST_INVITE_CODE", "INVALID-QA-CODE")}
    if "/members/" in path and path.endswith("/role"):
        return {"role": os.getenv("API_TEST_GROUP_ROLE", "member")}
    if path.endswith("/members/role-by-phone"):
        return {
            "phone": os.getenv("API_TEST_TARGET_PHONE", "+996700000002"),
            "role": os.getenv("API_TEST_GROUP_ROLE", "member"),
        }
    if "comment-mutes/by-phone" in path or "unmute-by-phone" in path:
        return {"phone": os.getenv("API_TEST_TARGET_PHONE", "+996700000002")}
    if path.endswith("/invite-user"):
        return {"mobile": os.getenv("API_TEST_TARGET_PHONE", "+996700000002")}
    if path.endswith("/messages"):
        return {"text": f"QA message {int(time.time())}"}
    if path.endswith("/requests") and "{groupID}" in path:
        return {
            "title": f"QA public request {int(time.time())}",
            "description": "Created by endpoint coverage test.",
            "request_type": "problem",
            "interaction_mode": "support_oppose",
            "category": "other",
            "priority": "normal",
        }
    if path.endswith("/requests/read"):
        return {}
    if path.endswith("/comments"):
        return {"text": f"QA comment {int(time.time())}"}
    if path.endswith("/status"):
        return {"status": os.getenv("API_TEST_REQUEST_STATUS", "in_progress")}
    if path.endswith("/hide"):
        return {"hidden": True}
    if path.endswith("/approve") or path.endswith("/reject") or path.endswith("/need-more-info"):
        return {"admin_comment": "QA endpoint coverage test."}
    if path == "/api/group-creation-requests":
        return {
            "applicant_name": "QA Tester",
            "position": "QA",
            "organization_name": "QA Organization",
            "organization_type": "community",
            "region": "Bishkek",
            "official_phone": "+996700000001",
            "official_email": "qa@example.com",
            "website": "https://example.com",
            "group_title": f"QA Requested Group {int(time.time())}",
            "group_description": "Created by endpoint coverage test.",
            "reason": "Automated QA coverage.",
            "documents": "Test-only request.",
        }
    return {}


def _path_with_fixtures(path: str) -> str:
    replacements = {
        "{groupID}": os.getenv("API_TEST_GROUP_ID", "missing-group-id"),
        "{userID}": os.getenv("API_TEST_TARGET_USER_ID", "missing-user-id"),
        "{inviteID}": os.getenv("API_TEST_INVITE_ID", "missing-invite-id"),
        "{requestID}": os.getenv("API_TEST_REQUEST_ID", "missing-request-id"),
        "{commentID}": os.getenv("API_TEST_COMMENT_ID", "missing-comment-id"),
        "{itemID}": os.getenv("API_TEST_MODERATION_ITEM_ID", "missing-moderation-item-id"),
    }

    if path.startswith("/api/admin/group-creation-requests/"):
        replacements["{requestID}"] = os.getenv(
            "API_TEST_ADMIN_GROUP_CREATION_REQUEST_ID",
            replacements["{requestID}"],
        )

    for key, value in replacements.items():
        path = path.replace(key, value)
    return path


def _missing_fixture_names(path: str) -> list[str]:
    required = {
        "{groupID}": "API_TEST_GROUP_ID",
        "{userID}": "API_TEST_TARGET_USER_ID",
        "{inviteID}": "API_TEST_INVITE_ID",
        "{requestID}": "API_TEST_REQUEST_ID",
        "{commentID}": "API_TEST_COMMENT_ID",
        "{itemID}": "API_TEST_MODERATION_ITEM_ID",
    }
    missing = [env for marker, env in required.items() if marker in path and not os.getenv(env)]
    if path.startswith("/api/admin/group-creation-requests/") and not os.getenv(
        "API_TEST_ADMIN_GROUP_CREATION_REQUEST_ID"
    ):
        missing.append("API_TEST_ADMIN_GROUP_CREATION_REQUEST_ID")
    return sorted(set(missing))


@pytest.fixture(scope="session")
def token(http_session: requests.Session, base_url: str) -> str:
    phone = os.getenv("API_TEST_PHONE", "+996700000001")
    code = os.getenv("API_TEST_CODE", "111111")
    display_name = os.getenv("API_TEST_DISPLAY_NAME", "API Test User")

    _request(
        http_session,
        base_url,
        "POST",
        "/api/auth/request-code",
        body={"mobile": phone},
        expected={200},
    )
    verify = _request(
        http_session,
        base_url,
        "POST",
        "/api/auth/verify-code",
        body={"mobile": phone, "code": code, "display_name": display_name},
        expected={200},
    )
    payload = verify.json()
    access_token = payload.get("access_token") or payload.get("token")
    assert access_token, f"verify-code did not return access token: {payload}"
    return access_token


def test_endpoint_manifest_matches_router_source() -> None:
    manifest = {(item.method, item.path) for item in _load_manifest()}
    discovered = _discover_routes_from_server_go()

    assert manifest == discovered, (
        "Endpoint manifest is out of sync with internal/httpapi/server.go.\n"
        f"Missing from manifest: {sorted(discovered - manifest)}\n"
        f"Removed from server.go but still in manifest: {sorted(manifest - discovered)}"
    )


@pytest.mark.parametrize("endpoint", [item for item in _load_manifest() if not item.auth], ids=lambda e: f"{e.method} {e.path}")
def test_public_endpoints(endpoint: Endpoint, http_session: requests.Session, base_url: str) -> None:
    path = _path_with_fixtures(endpoint.path)
    body = _body_for(endpoint)

    expected = {200}
    if endpoint.path == "/api/auth/logout":
        expected = {200, 401, 404}
    if endpoint.path == "/api/auth/refresh":
        expected = {200, 401, 404}

    _request(http_session, base_url, endpoint.method, path, body=body, expected=expected)


@pytest.mark.parametrize("endpoint", [item for item in _load_manifest() if item.auth], ids=lambda e: f"{e.method} {e.path}")
def test_every_protected_endpoint_rejects_missing_bearer_token(
    endpoint: Endpoint,
    http_session: requests.Session,
    base_url: str,
) -> None:
    path = _path_with_fixtures(endpoint.path)
    _request(http_session, base_url, endpoint.method, path, body=_body_for(endpoint), expected={401})


def test_authenticated_core_flow(http_session: requests.Session, base_url: str, token: str) -> None:
    core_calls = [
        Endpoint("GET", "/api/me", True, "profile"),
        Endpoint("POST", "/api/ws-token", True, "websocket"),
        Endpoint("POST", "/api/push/register", True, "push"),
        Endpoint("DELETE", "/api/push/token", True, "push"),
        Endpoint("GET", "/api/groups", True, "groups"),
        Endpoint("GET", "/api/groups/search", True, "groups"),
        Endpoint("GET", "/api/invites", True, "invites"),
        Endpoint("POST", "/api/group-creation-requests", True, "group_creation"),
        Endpoint("GET", "/api/group-creation-requests", True, "group_creation"),
    ]

    for endpoint in core_calls:
        path = _path_with_fixtures(endpoint.path)
        if path == "/api/groups/search":
            path = "/api/groups/search?q=qa"
        _request(
            http_session,
            base_url,
            endpoint.method,
            path,
            token=token,
            body=_body_for(endpoint),
            expected={200, 201, 202, 403},
        )


@pytest.mark.parametrize("endpoint", [item for item in _load_manifest() if item.auth and "{" in item.path], ids=lambda e: f"{e.method} {e.path}")
def test_fixture_based_resource_endpoints(
    endpoint: Endpoint,
    http_session: requests.Session,
    base_url: str,
    token: str,
) -> None:
    missing = _missing_fixture_names(endpoint.path)
    strict = os.getenv("API_STRICT_FULL", "0") == "1"
    if missing and strict:
        pytest.fail(f"{endpoint.method} {endpoint.path} needs fixture env vars: {', '.join(missing)}")
    if missing:
        pytest.skip(f"Fixture required for {endpoint.method} {endpoint.path}: {', '.join(missing)}")

    path = _path_with_fixtures(endpoint.path)
    expected = {200, 201, 202, 204, 400, 403, 404}
    response = _request(
        http_session,
        base_url,
        endpoint.method,
        path,
        token=token,
        body=_body_for(endpoint),
        expected=expected,
    )

    assert response.status_code != 500, f"{endpoint.method} {path} returned 500: {_json_or_text(response)!r}"
