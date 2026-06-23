CREATE TABLE IF NOT EXISTS learned_moderation_rules (
    id bigserial PRIMARY KEY,
    group_id text NOT NULL,
    pattern text NOT NULL,
    action text NOT NULL DEFAULT 'deny',
    weight integer NOT NULL DEFAULT 1,
    source_item_id text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(group_id, pattern)
);

CREATE INDEX IF NOT EXISTS idx_learned_moderation_rules_group_weight
    ON learned_moderation_rules (group_id, weight DESC, updated_at DESC);
