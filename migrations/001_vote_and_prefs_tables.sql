-- Adds tables that the code uses but older schema.sql versions omitted.
-- Safe to run more than once (IF NOT EXISTS). Apply to databases created
-- before this file existed:  psql "$DB_URL" -f migrations/001_vote_and_prefs_tables.sql

-- ── 13. board_flow_votes ──────────────────────────────────────────────────────
-- One vote per user per board flow (VotePanel). Upserted on
-- (board_id, flow_id, user_id); values match board_flow_vote_service.
CREATE TABLE IF NOT EXISTS public.board_flow_votes (
    board_id   TEXT        NOT NULL REFERENCES public.boards(id)      ON DELETE CASCADE,
    flow_id    TEXT        NOT NULL REFERENCES public.board_flows(id) ON DELETE CASCADE,
    user_id    TEXT        NOT NULL REFERENCES public.users(id)       ON DELETE CASCADE,
    vote       TEXT        NOT NULL CHECK (vote IN ('approve', 'review', 'reject')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (board_id, flow_id, user_id)
);

-- ── 14. user_notification_prefs ───────────────────────────────────────────────
-- Per-user notification settings; a missing row means the defaults in
-- model.DefaultNotificationPrefs (comments, invites on; product updates off).
CREATE TABLE IF NOT EXISTS public.user_notification_prefs (
    user_id         TEXT        PRIMARY KEY REFERENCES public.users(id) ON DELETE CASCADE,
    comments        BOOLEAN     NOT NULL DEFAULT TRUE,
    invites         BOOLEAN     NOT NULL DEFAULT TRUE,
    product_updates BOOLEAN     NOT NULL DEFAULT FALSE,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
