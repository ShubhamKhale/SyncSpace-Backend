-- SyncSpace public schema — run once before starting the server.
-- Tables are ordered by foreign-key dependency (referenced tables first).

-- ── 1. users ──────────────────────────────────────────────────────────────────
CREATE TABLE public.users (
    id            TEXT        PRIMARY KEY,
    name          TEXT        NOT NULL DEFAULT '',
    email         TEXT        NOT NULL UNIQUE,
    bio           TEXT        NOT NULL DEFAULT '',
    avatar_url    TEXT        NOT NULL DEFAULT '',
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── 2. organizations ──────────────────────────────────────────────────────────
CREATE TABLE public.organizations (
    id          TEXT        PRIMARY KEY,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    owner_id    TEXT        NOT NULL REFERENCES public.users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── 3. organization_members ───────────────────────────────────────────────────
-- Valid roles:    admin | member | viewer   (owner is tracked via organizations.owner_id)
-- Valid statuses: active | invited
CREATE TABLE public.organization_members (
    organization_id TEXT        NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    user_id         TEXT        NOT NULL REFERENCES public.users(id)         ON DELETE CASCADE,
    role            TEXT        NOT NULL CHECK (role IN ('admin', 'member', 'viewer')),
    status          TEXT        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'invited')),
    invited_by      TEXT        REFERENCES public.users(id),
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_id, user_id)
);

-- ── 3b. invite_tokens ────────────────────────────────────────────────────────
CREATE TABLE public.invite_tokens (
    id          TEXT        PRIMARY KEY,
    token       TEXT        NOT NULL UNIQUE,
    org_id      TEXT        NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    email       TEXT        NOT NULL,
    role        TEXT        NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member', 'viewer')),
    invited_by  TEXT        NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '7 days'),
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invite_tokens_token ON public.invite_tokens(token);
CREATE INDEX idx_invite_tokens_email ON public.invite_tokens(email);

-- ── 4. boards ─────────────────────────────────────────────────────────────────
CREATE TABLE public.boards (
    id          TEXT        PRIMARY KEY,
    title       TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    owner_id    TEXT        NOT NULL REFERENCES public.users(id),
    org_id      TEXT        REFERENCES public.organizations(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_boards_org_id ON public.boards(org_id);

-- ── 5. tasks ──────────────────────────────────────────────────────────────────
-- Valid stages:     todo | in_progress | done
-- Valid priorities: low | medium | high
CREATE TABLE public.tasks (
    id          TEXT        PRIMARY KEY,
    board_id    TEXT        NOT NULL REFERENCES public.boards(id) ON DELETE CASCADE,
    title       TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    stage       TEXT        NOT NULL DEFAULT 'Planning',
    priority    TEXT        NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high')),
    assignee_id TEXT        REFERENCES public.users(id),
    created_by  TEXT        NOT NULL REFERENCES public.users(id),
    position    INTEGER     NOT NULL DEFAULT 0,
    due_date    TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tasks_board     ON public.tasks(board_id);
CREATE INDEX idx_tasks_assignee  ON public.tasks(assignee_id);
CREATE INDEX idx_tasks_due_date  ON public.tasks(due_date) WHERE due_date IS NOT NULL;

-- ── 6. notifications ──────────────────────────────────────────────────────────
CREATE TABLE public.notifications (
    id          TEXT        PRIMARY KEY,
    user_id     TEXT        NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    title       TEXT        NOT NULL,
    body        TEXT        NOT NULL DEFAULT '',
    is_read     BOOLEAN     NOT NULL DEFAULT FALSE,
    entity_type TEXT,
    entity_id   TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user    ON public.notifications(user_id);
CREATE INDEX idx_notifications_unread  ON public.notifications(user_id) WHERE is_read = FALSE;

-- ── 7. activity_logs ──────────────────────────────────────────────────────────
CREATE TABLE public.activity_logs (
    id          TEXT        PRIMARY KEY,
    entity_type TEXT        NOT NULL,
    entity_id   TEXT        NOT NULL,
    actor_id    TEXT        NOT NULL REFERENCES public.users(id),
    action      TEXT        NOT NULL,
    metadata    JSONB       NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_activity_entity  ON public.activity_logs(entity_id);
CREATE INDEX idx_activity_actor   ON public.activity_logs(actor_id);

-- ── 8. linked_resources ───────────────────────────────────────────────────────
CREATE TABLE public.linked_resources (
    id         TEXT        PRIMARY KEY,
    board_id   TEXT        NOT NULL REFERENCES public.boards(id) ON DELETE CASCADE,
    label      TEXT        NOT NULL,
    url        TEXT        NOT NULL,
    created_by TEXT        NOT NULL REFERENCES public.users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_linked_resources_board ON public.linked_resources(board_id);

-- ── 9. flows ──────────────────────────────────────────────────────────────────
CREATE TABLE public.flows (
    id               TEXT        PRIMARY KEY,
    board_id         TEXT        NOT NULL REFERENCES public.boards(id) ON DELETE CASCADE,
    title            TEXT        NOT NULL DEFAULT '',
    data             JSONB       NOT NULL DEFAULT '{"nodes":[],"edges":[]}',
    version          INTEGER     NOT NULL DEFAULT 0,
    last_modified_by TEXT        NOT NULL REFERENCES public.users(id),
    created_by       TEXT        NOT NULL REFERENCES public.users(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_flows_board ON public.flows(board_id);

-- ── 10. flow_votes ────────────────────────────────────────────────────────────
CREATE TABLE public.flow_votes (
    flow_id    TEXT        NOT NULL REFERENCES public.flows(id) ON DELETE CASCADE,
    user_id    TEXT        NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    vote_type  TEXT        NOT NULL CHECK (vote_type IN ('up', 'down')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (flow_id, user_id)
);

-- ── 11. board_flows ──────────────────────────────────────────────────────────
-- Multi-flow diagrams per board (metadata only; canvas data in flow_diagrams).
CREATE TABLE public.board_flows (
  id          TEXT        PRIMARY KEY,
  board_id    TEXT        NOT NULL REFERENCES public.boards(id) ON DELETE CASCADE,
  name        TEXT        NOT NULL DEFAULT 'Untitled Flow',
  node_count  INT         NOT NULL DEFAULT 0,
  edge_count  INT         NOT NULL DEFAULT 0,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_board_flows_board_id      ON public.board_flows(board_id);
CREATE INDEX idx_board_flows_board_updated ON public.board_flows(board_id, updated_at DESC);

CREATE TABLE public.flow_diagrams (
  flow_id    TEXT        PRIMARY KEY REFERENCES public.board_flows(id) ON DELETE CASCADE,
  diagram    JSONB       NOT NULL DEFAULT '{"title":"Untitled Diagram","nodes":[],"edges":[]}',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── 12. email_templates ───────────────────────────────────────────────────────
CREATE TABLE public.email_templates (
    id         TEXT        PRIMARY KEY,
    name       TEXT        UNIQUE NOT NULL,
    subject    TEXT        NOT NULL,
    html_body  TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO public.email_templates (id, name, subject, html_body, created_at, updated_at) VALUES (
    'tpl-org-invite',
    'org_invite',
    'You''ve been invited to join {{.OrgName}} on SyncSpace',
    '<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;max-width:560px;margin:40px auto;color:#1a1a1a;">
  <h2 style="margin-bottom:4px;">You have been invited!</h2>
  <p>{{.InviterName}} has invited you to join <strong>{{.OrgName}}</strong> as <strong>{{.Role}}</strong>.</p>
  <p style="margin:24px 0;">
    <a href="{{.InviteLink}}"
       style="background:#6366f1;color:#fff;padding:12px 24px;border-radius:6px;text-decoration:none;font-weight:600;">
      Accept Invitation
    </a>
  </p>
  <p style="color:#555;font-size:14px;">This link expires in 7 days.</p>
  <p style="color:#555;font-size:14px;">If you did not expect this invitation, you can safely ignore this email.</p>
</body>
</html>',
    NOW(), NOW()
);
