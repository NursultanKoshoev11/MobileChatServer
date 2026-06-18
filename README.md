# Koom Server

Koom Server is the backend API for the Koom civic communication platform.

It powers phone-based authentication, groups, invite codes, public requests, voting, comments, request statuses, group statistics, push token registration, and realtime group communication.

## Content moderation

The backend moderates user-generated content before it is published. This is enforced server-side for:

- group messages;
- public requests / publications;
- public request comments.

Moderation has three layers:

1. local rules for profanity, suspicious advertising patterns, repeated spam-like text, links and phone contacts;
2. optional OpenAI Moderation API using `OPENAI_API_KEY` and `OPENAI_MODERATION_MODEL`;
3. manual admin review queue for content that should not be published automatically.

When content needs review, the create endpoint returns HTTP `202` with `status: "pending_review"` and a `moderation_item`. Group owners/admins can review pending items:

- `GET /api/groups/{groupID}/moderation/items?status=pending`
- `POST /api/moderation/items/{itemID}/approve`
- `POST /api/moderation/items/{itemID}/reject`

Approved items are published by the backend and broadcast to connected clients through the existing realtime events.
