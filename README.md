# Koom Server

Koom Server is the backend API for the Koom civic communication platform.

It powers phone-based authentication, groups, invite codes, public requests, voting, comments, request statuses, group statistics, push token registration, and realtime group communication.

## Content moderation

The backend moderates user-generated content before it is published. This is enforced server-side for:

- group messages;
- public requests / publications;
- public request comments.

Moderation has four layers:

1. free local rules for Kyrgyz, Russian, and English text;
2. optional free-tier Hugging Face Inference API via `HF_TOKEN`;
3. optional OpenAI Moderation API via `OPENAI_API_KEY`;
4. manual admin review queue for content that should not be published automatically.

Provider selection is controlled with:

- `CONTENT_MODERATION_PROVIDER=local` for local rules only;
- `CONTENT_MODERATION_PROVIDER=huggingface` for local rules plus Hugging Face when `HF_TOKEN` is configured;
- `CONTENT_MODERATION_PROVIDER=openai` for local rules plus OpenAI when `OPENAI_API_KEY` is configured.

If Hugging Face or OpenAI is not configured, the backend still works with the built-in local rules. Kyrgyz text is normalized for letters such as `ү`, `ң`, and `ө`, and common Kyrgyz advertising / abusive-language patterns are sent to manual review.

When content needs review, the create endpoint returns HTTP `202` with `status: "pending_review"` and a `moderation_item`. Group owners/admins can review pending items:

- `GET /api/groups/{groupID}/moderation/items?status=pending`
- `POST /api/moderation/items/{itemID}/approve`
- `POST /api/moderation/items/{itemID}/reject`

Approved items are published by the backend and broadcast to connected clients through the existing realtime events.
