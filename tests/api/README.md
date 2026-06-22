# API endpoint coverage tests

This folder is for full MobileChatServer API endpoint testing.

## What it checks

1. **Endpoint inventory gate** — `test_endpoint_manifest_matches_router_source` parses `internal/httpapi/server.go` and compares the actual `chi` routes with `tests/api/endpoints_manifest.json`.
   - If a new endpoint is added in code but not added to the manifest/tests, the test fails.
   - This prevents fake "100% endpoint coverage".

2. **Public endpoint checks** — health and auth endpoints are called directly.

3. **Protected endpoint checks** — every protected endpoint is called without a bearer token and must return `401`, proving the route exists and is protected.

4. **Authenticated core flow** — login with test auth, then calls stable authenticated endpoints:
   - `/api/me`
   - `/api/ws-token`
   - `/api/push/register`
   - `/api/push/token`
   - `/api/groups`
   - `/api/groups/search`
   - `/api/invites`
   - `/api/group-creation-requests`

5. **Fixture-based endpoint checks** — resource-specific endpoints are called when fixture IDs are provided.

## Local run

```bash
python -m pip install -r tests/api/requirements.txt

API_BASE_URL=http://127.0.0.1:8080 \
API_TEST_PHONE=+996700000001 \
API_TEST_CODE=111111 \
python -m pytest tests/api -v
```

## Strict 100% business run

For a strict pass across resource-specific endpoints, set `API_STRICT_FULL=1` and provide all fixture IDs:

```bash
API_BASE_URL=https://your-staging-api.example.com \
API_TEST_PHONE=+996700000001 \
API_TEST_CODE=111111 \
API_TEST_GROUP_ID=<group-id> \
API_TEST_TARGET_USER_ID=<target-user-id> \
API_TEST_INVITE_ID=<invite-id> \
API_TEST_REQUEST_ID=<public-request-id> \
API_TEST_COMMENT_ID=<comment-id> \
API_TEST_MODERATION_ITEM_ID=<moderation-item-id> \
API_TEST_ADMIN_GROUP_CREATION_REQUEST_ID=<admin-group-creation-request-id> \
API_STRICT_FULL=1 \
python -m pytest tests/api -v
```

## Important

Without fixture IDs, the suite still verifies endpoint inventory, public routes, auth flow, and `401` protection for every protected endpoint.  
Strict business-level validation of endpoints that require existing group/request/comment/invite/moderation IDs is not possible honestly without test fixtures.
