-- Per-user, per-flow AI conversation history.
-- Stores the full conversation state so users can continue where they left off.
-- Safe to run more than once (IF NOT EXISTS). Apply:
--   psql "$DB_URL" -f migrations/002_flow_ai_conversations.sql

CREATE TABLE IF NOT EXISTS public.flow_ai_conversations (
    user_id    TEXT        NOT NULL REFERENCES public.users(id)        ON DELETE CASCADE,
    flow_id    TEXT        NOT NULL REFERENCES public.board_flows(id)  ON DELETE CASCADE,
    history    JSONB       NOT NULL DEFAULT '[]',
    messages   JSONB       NOT NULL DEFAULT '[]',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, flow_id)
);
