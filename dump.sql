-- SyncSpace database dump
-- Restore: psql $NEW_DB_URL < dump.sql

SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;

-- ========== SCHEMA ==========

CREATE TABLE IF NOT EXISTS public.activity_logs (
    id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT activity_logs_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES public.users (id),
    PRIMARY KEY (id)
);
CREATE INDEX idx_activity_entity ON public.activity_logs USING btree (entity_id);
CREATE INDEX idx_activity_actor ON public.activity_logs USING btree (actor_id);
CREATE INDEX idx_activity_logs_created_at ON public.activity_logs USING btree (created_at DESC);

CREATE TABLE IF NOT EXISTS public.board_flow_votes (
    board_id TEXT NOT NULL,
    flow_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    vote TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((vote = ANY (ARRAY['approve'::text, 'review'::text, 'reject'::text]))),
    CONSTRAINT board_flow_votes_board_id_fkey FOREIGN KEY (board_id) REFERENCES public.boards (id) ON DELETE CASCADE,
    CONSTRAINT board_flow_votes_flow_id_fkey FOREIGN KEY (flow_id) REFERENCES public.board_flows (id) ON DELETE CASCADE,
    CONSTRAINT board_flow_votes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE,
    PRIMARY KEY (board_id, flow_id, user_id)
);

CREATE TABLE IF NOT EXISTS public.board_flows (
    id TEXT NOT NULL,
    board_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT 'Untitled Flow'::text,
    node_count INTEGER NOT NULL DEFAULT 0,
    edge_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT board_flows_board_id_fkey FOREIGN KEY (board_id) REFERENCES public.boards (id) ON DELETE CASCADE,
    PRIMARY KEY (id)
);
CREATE INDEX idx_board_flows_board_id ON public.board_flows USING btree (board_id);
CREATE INDEX idx_board_flows_board_updated ON public.board_flows USING btree (board_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS public.boards (
    id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT ''::text,
    owner_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    org_id TEXT,
    CONSTRAINT boards_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations (id) ON DELETE CASCADE,
    CONSTRAINT boards_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public.users (id),
    PRIMARY KEY (id)
);
CREATE INDEX idx_boards_org_id ON public.boards USING btree (org_id);

CREATE TABLE IF NOT EXISTS public.email_templates (
    id TEXT NOT NULL,
    name TEXT NOT NULL,
    subject TEXT NOT NULL,
    html_body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT email_templates_name_key UNIQUE (name),
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS public.flow_diagrams (
    flow_id TEXT NOT NULL,
    diagram JSONB NOT NULL DEFAULT '{"edges": [], "nodes": [], "title": "Untitled Diagram"}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (flow_id)
);

CREATE TABLE IF NOT EXISTS public.flow_votes (
    flow_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    vote_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((vote_type = ANY (ARRAY['up'::text, 'down'::text]))),
    CONSTRAINT flow_votes_flow_id_fkey FOREIGN KEY (flow_id) REFERENCES public.flows (id) ON DELETE CASCADE,
    CONSTRAINT flow_votes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE,
    PRIMARY KEY (flow_id, user_id)
);

CREATE TABLE IF NOT EXISTS public.flows (
    id TEXT NOT NULL,
    board_id TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT ''::text,
    data JSONB NOT NULL DEFAULT '{"edges": [], "nodes": []}'::jsonb,
    version INTEGER NOT NULL DEFAULT 0,
    last_modified_by TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT flows_board_id_fkey FOREIGN KEY (board_id) REFERENCES public.boards (id) ON DELETE CASCADE,
    CONSTRAINT flows_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users (id),
    CONSTRAINT flows_last_modified_by_fkey FOREIGN KEY (last_modified_by) REFERENCES public.users (id),
    PRIMARY KEY (id)
);
CREATE INDEX idx_flows_board ON public.flows USING btree (board_id);

CREATE TABLE IF NOT EXISTS public.invite_tokens (
    id TEXT NOT NULL,
    token TEXT NOT NULL,
    org_id TEXT NOT NULL,
    email TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member'::text,
    invited_by TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + '7 days'::interval),
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((role = ANY (ARRAY['admin'::text, 'member'::text, 'viewer'::text]))),
    CONSTRAINT invite_tokens_token_key UNIQUE (token),
    CONSTRAINT invite_tokens_invited_by_fkey FOREIGN KEY (invited_by) REFERENCES public.users (id) ON DELETE CASCADE,
    CONSTRAINT invite_tokens_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations (id) ON DELETE CASCADE,
    PRIMARY KEY (id)
);
CREATE INDEX idx_invite_tokens_token ON public.invite_tokens USING btree (token);
CREATE INDEX idx_invite_tokens_email ON public.invite_tokens USING btree (email);

CREATE TABLE IF NOT EXISTS public.linked_resources (
    id TEXT NOT NULL,
    board_id TEXT NOT NULL,
    label TEXT NOT NULL,
    url TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT linked_resources_board_id_fkey FOREIGN KEY (board_id) REFERENCES public.boards (id) ON DELETE CASCADE,
    CONSTRAINT linked_resources_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users (id),
    PRIMARY KEY (id)
);
CREATE INDEX idx_linked_resources_board ON public.linked_resources USING btree (board_id);

CREATE TABLE IF NOT EXISTS public.notifications (
    id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT ''::text,
    is_read BOOLEAN NOT NULL DEFAULT false,
    entity_type TEXT,
    entity_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT notifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE,
    PRIMARY KEY (id)
);
CREATE INDEX idx_notifications_user ON public.notifications USING btree (user_id);
CREATE INDEX idx_notifications_unread ON public.notifications USING btree (user_id) WHERE (is_read = false);

CREATE TABLE IF NOT EXISTS public.organization_members (
    organization_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    role TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'::text,
    invited_by TEXT,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((status = ANY (ARRAY['active'::text, 'invited'::text]))),
    CHECK ((role = ANY (ARRAY['owner'::text, 'admin'::text, 'member'::text, 'viewer'::text]))),
    CONSTRAINT organization_members_invited_by_fkey FOREIGN KEY (invited_by) REFERENCES public.users (id),
    CONSTRAINT organization_members_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES public.organizations (id) ON DELETE CASCADE,
    CONSTRAINT organization_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE,
    PRIMARY KEY (organization_id, user_id)
);
CREATE INDEX idx_org_members_user_status ON public.organization_members USING btree (user_id, status);

CREATE TABLE IF NOT EXISTS public.organizations (
    id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT ''::text,
    owner_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT organizations_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public.users (id),
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS public.tasks (
    id TEXT NOT NULL,
    board_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT ''::text,
    stage TEXT NOT NULL DEFAULT 'Planning'::text,
    priority TEXT NOT NULL DEFAULT 'medium'::text,
    assignee_id TEXT,
    created_by TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    due_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    start_date TIMESTAMPTZ,
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    time_estimate TEXT NOT NULL DEFAULT ''::text,
    reference_link TEXT NOT NULL DEFAULT ''::text,
    flow_diagram_link TEXT NOT NULL DEFAULT ''::text,
    subtasks JSONB NOT NULL DEFAULT '[]'::jsonb,
    attachments JSONB NOT NULL DEFAULT '[]'::jsonb,
    CHECK ((priority = ANY (ARRAY['low'::text, 'medium'::text, 'high'::text]))),
    CONSTRAINT tasks_assignee_id_fkey FOREIGN KEY (assignee_id) REFERENCES public.users (id),
    CONSTRAINT tasks_board_id_fkey FOREIGN KEY (board_id) REFERENCES public.boards (id) ON DELETE CASCADE,
    CONSTRAINT tasks_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users (id),
    PRIMARY KEY (id)
);
CREATE INDEX idx_tasks_board ON public.tasks USING btree (board_id);
CREATE INDEX idx_tasks_assignee ON public.tasks USING btree (assignee_id);
CREATE INDEX idx_tasks_due_date ON public.tasks USING btree (due_date) WHERE (due_date IS NOT NULL);
CREATE INDEX idx_tasks_assignee_due ON public.tasks USING btree (assignee_id, due_date) WHERE ((due_date IS NOT NULL) AND (stage <> 'done'::text));
CREATE INDEX idx_tasks_created_by_due ON public.tasks USING btree (created_by, due_date) WHERE ((due_date IS NOT NULL) AND (stage <> 'done'::text));

CREATE TABLE IF NOT EXISTS public.user_notification_prefs (
    user_id TEXT NOT NULL,
    comments BOOLEAN NOT NULL DEFAULT true,
    invites BOOLEAN NOT NULL DEFAULT true,
    product_updates BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT user_notification_prefs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id)
);

CREATE TABLE IF NOT EXISTS public.users (
    id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT ''::text,
    email TEXT NOT NULL,
    bio TEXT NOT NULL DEFAULT ''::text,
    avatar_url TEXT NOT NULL DEFAULT ''::text,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT users_email_key UNIQUE (email),
    PRIMARY KEY (id)
);

-- ========== DATA ==========

-- activity_logs
INSERT INTO public.activity_logs (id, entity_type, entity_id, actor_id, action, metadata, created_at) VALUES ('41c0c33a-a812-46a4-baf5-a02b095db8ad', 'board', '346ff099-6f51-4c95-a951-1f3e7b755f88', '1de56956-a6ea-4fbe-843b-2125339399ed', 'created', '{"title":"Product Roadmap"}', '2026-06-26 13:39:56.521876+00') ON CONFLICT DO NOTHING;
INSERT INTO public.activity_logs (id, entity_type, entity_id, actor_id, action, metadata, created_at) VALUES ('396b54be-dcfc-47b2-aaaa-6c3df0d455f2', 'task', '06b78371-0752-4f48-bc93-2769de30f4cc', '1de56956-a6ea-4fbe-843b-2125339399ed', 'created', '{"board_id":"346ff099-6f51-4c95-a951-1f3e7b755f88","stage":"Planning","title":"Implement login page UI"}', '2026-07-11 09:49:56.279469+00') ON CONFLICT DO NOTHING;
INSERT INTO public.activity_logs (id, entity_type, entity_id, actor_id, action, metadata, created_at) VALUES ('0bf5218f-0575-4b96-8002-e2caf8093a47', 'task', '1081ef59-f46f-42bb-93d5-eb2c837c156b', '1de56956-a6ea-4fbe-843b-2125339399ed', 'created', '{"board_id":"346ff099-6f51-4c95-a951-1f3e7b755f88","stage":"Design","title":"Design onboarding flow screens"}', '2026-07-11 10:15:59.428162+00') ON CONFLICT DO NOTHING;
INSERT INTO public.activity_logs (id, entity_type, entity_id, actor_id, action, metadata, created_at) VALUES ('3c739c9d-9059-4754-ba63-b7d395d2f478', 'board', '346ff099-6f51-4c95-a951-1f3e7b755f88', '1de56956-a6ea-4fbe-843b-2125339399ed', 'updated', '{"field":"linked_resources","resource_count":2}', '2026-07-13 07:04:20.448099+00') ON CONFLICT DO NOTHING;

-- board_flow_votes
INSERT INTO public.board_flow_votes (board_id, flow_id, user_id, vote, created_at, updated_at) VALUES ('346ff099-6f51-4c95-a951-1f3e7b755f88', '7e477cfc-668d-411e-a40d-bf5b970151dd', '1de56956-a6ea-4fbe-843b-2125339399ed', 'review', '2026-07-19 11:14:51.113477+00', '2026-07-19 11:14:55.038773+00') ON CONFLICT DO NOTHING;

-- board_flows
INSERT INTO public.board_flows (id, board_id, name, node_count, edge_count, created_at, updated_at) VALUES ('7e477cfc-668d-411e-a40d-bf5b970151dd', '346ff099-6f51-4c95-a951-1f3e7b755f88', 'Untitled Flow 1', 8, 7, '2026-07-13 12:45:02.192546+00', '2026-07-19 11:30:48.645344+00') ON CONFLICT DO NOTHING;

-- boards
INSERT INTO public.boards (id, title, description, owner_id, created_at, updated_at, org_id) VALUES ('346ff099-6f51-4c95-a951-1f3e7b755f88', 'Product Roadmap', 'This is the sample board for adding product roadmap', '1de56956-a6ea-4fbe-843b-2125339399ed', '2026-06-26 13:39:55.215642+00', '2026-06-27 11:47:02.96665+00', 'f2ef71f0-aaa1-454a-87c5-2614382896b3') ON CONFLICT DO NOTHING;

-- email_templates
INSERT INTO public.email_templates (id, name, subject, html_body, created_at, updated_at) VALUES ('tpl-org-invite', 'org_invite', 'You''ve been invited to join {{.OrgName}} on SyncSpace', '<!DOCTYPE html>
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
</html>', '2026-06-27 17:04:09.278829+00', '2026-06-27 17:04:09.278829+00') ON CONFLICT DO NOTHING;

-- flow_diagrams
INSERT INTO public.flow_diagrams (flow_id, diagram, updated_at) VALUES ('5bfe6804-9df9-4bce-8e29-315a8c825084', '{"edges":[],"nodes":[],"title":"Untitled Diagram"}', '2026-07-13 12:01:37.753286+00') ON CONFLICT DO NOTHING;
INSERT INTO public.flow_diagrams (flow_id, diagram, updated_at) VALUES ('1639299c-416e-400e-a8cd-4e488d3948b2', '{"edges":[],"nodes":[],"title":"Untitled Diagram"}', '2026-07-13 12:01:37.75394+00') ON CONFLICT DO NOTHING;
INSERT INTO public.flow_diagrams (flow_id, diagram, updated_at) VALUES ('4bbbd05f-5e6a-4dda-9c26-54c83e50ed0c', '{"edges":[],"nodes":[],"title":"Untitled Diagram"}', '2026-07-13 12:01:39.521601+00') ON CONFLICT DO NOTHING;
INSERT INTO public.flow_diagrams (flow_id, diagram, updated_at) VALUES ('c4d60a00-cfe3-4546-a75f-aff442162934', '{"edges":[{"animated":true,"data":{"algorithm":"smart","animation":"animatedDotted"},"id":"uje-1","selected":false,"source":"uj-stage1","style":{"animation":"dashdraw 0.4s linear infinite","stroke":"#CF4C2C","strokeDashArray":1000,"strokeDashOffset":1000},"target":"uj-stage2","type":"smoothstep"},{"animated":true,"data":{"algorithm":"smart","animation":"animatedDotted"},"id":"uje-2","selected":false,"source":"uj-stage2","style":{"animation":"dashdraw 0.4s linear infinite","stroke":"#CF4C2C","strokeDashArray":1000,"strokeDashOffset":1000},"target":"uj-stage3","type":"smoothstep"},{"animated":true,"data":{"algorithm":"smart","animation":"animatedDotted"},"id":"uje-3","selected":false,"source":"uj-stage3","style":{"animation":"dashdraw 0.4s linear infinite","stroke":"#CF4C2C","strokeDashArray":1000,"strokeDashOffset":1000},"target":"uj-stage4","type":"smoothstep"},{"id":"xy-edge__uj-stage1left-source-uj-stage1left-target","selected":false,"source":"uj-stage1","sourceHandle":"left-source","style":{"strokeWidth":2},"target":"uj-stage1","targetHandle":"left-target","type":"editable-edge"}],"nodes":[{"data":{"color":"#ffffff","fill":"#6366f1","fontSize":"13px","fontWeight":"bold","height":50,"text":"Awareness","type":"round-rectangle","width":153},"dragging":false,"height":50,"id":"uj-stage1","measured":{"height":50,"width":153},"position":{"x":-116.28880693612831,"y":39.07868564350889},"resizing":false,"selected":false,"style":{"height":50,"width":120},"type":"shape","width":153},{"data":{"color":"#ffffff","fill":"#3b82f6","fontSize":"13px","fontWeight":"bold","height":50,"text":"Consideration","type":"round-rectangle","width":162},"dragging":false,"height":50,"id":"uj-stage2","measured":{"height":50,"width":162},"position":{"x":101.42460268841987,"y":-60.56057880109033},"resizing":false,"selected":false,"style":{"height":50,"width":120},"type":"shape","width":162,"zIndex":0},{"data":{"color":"#ffffff","fill":"#10b981","fontSize":"13px","fontWeight":"bold","text":"Decision","type":"round-rectangle"},"dragging":false,"id":"uj-stage3","measured":{"height":50,"width":120},"position":{"x":340,"y":21.573712870177893},"selected":false,"style":{"height":50,"width":120},"type":"shape"},{"data":{"color":"#ffffff","fill":"#f59e0b","fontSize":"13px","fontWeight":"bold","text":"Retention","type":"round-rectangle"},"dragging":false,"id":"uj-stage4","measured":{"height":50,"width":120},"position":{"x":602.1115445557865,"y":-46.74086345372375},"selected":false,"style":{"height":50,"width":120},"type":"shape"},{"data":{"color":"#ede9fe","text":"Sees social media ad\n\n😐 Neutral"},"dragging":false,"id":"uj-t1","measured":{"height":90,"width":120},"position":{"x":-107.32663862173695,"y":112.87152618466715},"selected":false,"style":{"height":90,"width":120},"type":"sticky-note","zIndex":0},{"data":{"color":"#dbeafe","text":"Reads reviews\n\n🤔 Curious"},"dragging":false,"id":"uj-t2","measured":{"height":90,"width":120},"position":{"x":-70.39040594525284,"y":-117.83356189610731},"selected":false,"style":{"height":90,"width":120},"type":"sticky-note"},{"data":{"color":"#dcfce7","text":"Free trial signup\n\n😊 Excited"},"dragging":false,"id":"uj-t3","measured":{"height":90,"width":120},"position":{"x":300,"y":126.31474257403556},"selected":false,"style":{"height":90,"width":120},"type":"sticky-note"},{"data":{"color":"#fef9c3","text":"Weekly usage\n\n😄 Satisfied"},"dragging":false,"id":"uj-t4","measured":{"height":90,"width":120},"position":{"x":691.4790371354237,"y":-117.83356189610731},"selected":false,"style":{"height":90,"width":120},"type":"sticky-note"},{"data":{"color":"#fce7f3","text":"⚠️ Pain Points: Pricing unclear, onboarding confusing, slow load times"},"id":"uj-pain","measured":{"height":80,"width":280},"position":{"x":220,"y":270},"selected":false,"style":{"height":80,"width":280},"type":"sticky-note"}],"title":"Product Roadmap"}', '2026-07-13 12:01:40.583938+00') ON CONFLICT DO NOTHING;
INSERT INTO public.flow_diagrams (flow_id, diagram, updated_at) VALUES ('d8eef4bc-9167-4b9b-a2fb-6100c890b4e4', '{"edges":[],"nodes":[],"title":"Untitled Diagram"}', '2026-07-13 12:01:48.224809+00') ON CONFLICT DO NOTHING;
INSERT INTO public.flow_diagrams (flow_id, diagram, updated_at) VALUES ('7e477cfc-668d-411e-a40d-bf5b970151dd', '{"edges":[{"id":"sae-1","selected":false,"source":"sa-client","target":"sa-gateway","type":"smoothstep"},{"id":"sae-2","selected":false,"source":"sa-gateway","target":"sa-auth","type":"smoothstep"},{"id":"sae-3","selected":false,"source":"sa-gateway","target":"sa-api","type":"smoothstep"},{"id":"sae-4","selected":false,"source":"sa-gateway","target":"sa-worker","type":"smoothstep"},{"id":"sae-5","selected":false,"source":"sa-api","target":"sa-db","type":"smoothstep"},{"id":"sae-6","selected":false,"source":"sa-api","target":"sa-cache","type":"smoothstep"},{"id":"sae-7","selected":false,"source":"sa-worker","target":"sa-storage","type":"smoothstep"}],"nodes":[{"data":{"color":"#ffffff","fill":"#8b5cf6","fontSize":"13px","text":"API Gateway","type":"rectangle"},"id":"sa-gateway","measured":{"height":56,"width":140},"position":{"x":280,"y":140},"selected":false,"style":{"height":56,"width":140},"type":"shape"},{"data":{"color":"#ffffff","fill":"#f59e0b","fontSize":"12px","text":"Auth Service","type":"rectangle"},"id":"sa-auth","measured":{"height":56,"width":120},"position":{"x":80,"y":260},"selected":false,"style":{"height":56,"width":120},"type":"shape"},{"data":{"color":"#ffffff","fill":"#10b981","fontSize":"12px","text":"Core API\n(Node.js)","type":"rectangle"},"id":"sa-api","measured":{"height":56,"width":140},"position":{"x":280,"y":260},"selected":false,"style":{"height":56,"width":140},"type":"shape"},{"data":{"color":"#ffffff","fill":"#f59e0b","fontSize":"12px","text":"Worker\nService","type":"rectangle"},"id":"sa-worker","measured":{"height":56,"width":120},"position":{"x":480,"y":260},"selected":false,"style":{"height":56,"width":120},"type":"shape"},{"data":{"color":"#94a3b8","fill":"#1e293b","fontSize":"12px","height":60,"text":"PostgreSQL","type":"cylinder","width":134},"height":60,"id":"sa-db","measured":{"height":60,"width":134},"position":{"x":186,"y":390},"resizing":false,"selected":false,"style":{"height":60,"width":120},"type":"shape","width":134},{"data":{"color":"#ffffff","fill":"#dc2626","fontSize":"12px","text":"Redis Cache","type":"cylinder"},"id":"sa-cache","measured":{"height":60,"width":120},"position":{"x":380,"y":390},"selected":false,"style":{"height":60,"width":120},"type":"shape"},{"data":{"color":"#ffffff","fill":"#d97706","fontSize":"12px","text":"S3 Storage","type":"rectangle"},"id":"sa-storage","measured":{"height":56,"width":120},"position":{"x":560,"y":390},"selected":false,"style":{"height":56,"width":120},"type":"shape"},{"data":{"color":"#fef9c3","text":"Test Note"},"id":"1783946818983","measured":{"height":100,"width":100},"position":{"x":121.23703809912612,"y":135.0847652053204},"selected":false,"style":{"height":100,"width":100},"type":"sticky-note"}],"title":"Product Roadmap"}', '2026-07-19 11:30:48.037247+00') ON CONFLICT DO NOTHING;
INSERT INTO public.flow_diagrams (flow_id, diagram, updated_at) VALUES ('8f2db0a1-99af-4a04-bcf2-b33bbbea6880', '{"edges":[{"id":"fce-1","source":"fc-start","target":"fc-process1","type":"smoothstep"},{"id":"fce-2","source":"fc-process1","target":"fc-process2","type":"smoothstep"},{"id":"fce-3","source":"fc-process2","target":"fc-decision","type":"smoothstep"},{"id":"fce-4","label":"No","source":"fc-decision","target":"fc-fix","type":"smoothstep"},{"id":"fce-5","source":"fc-fix","target":"fc-process2","type":"smoothstep"},{"id":"fce-6","label":"Yes","source":"fc-decision","target":"fc-end","type":"smoothstep"}],"nodes":[{"data":{"color":"#ffffff","fill":"#10b981","fontSize":"14px","fontWeight":"bold","text":"Start","type":"round-rectangle"},"id":"fc-start","measured":{"height":50,"width":120},"position":{"x":300,"y":40},"style":{"height":50,"width":120},"type":"shape"},{"data":{"color":"#ffffff","fill":"#3b82f6","fontSize":"13px","text":"Define Requirements","type":"rectangle"},"id":"fc-process1","measured":{"height":56,"width":140},"position":{"x":300,"y":150},"style":{"height":56,"width":140},"type":"shape"},{"data":{"color":"#ffffff","fill":"#3b82f6","fontSize":"13px","text":"Build Solution","type":"rectangle"},"id":"fc-process2","measured":{"height":56,"width":140},"position":{"x":300,"y":270},"style":{"height":56,"width":140},"type":"shape"},{"data":{"color":"#ffffff","fill":"#f59e0b","fontSize":"13px","text":"Tests Pass?","type":"diamond"},"id":"fc-decision","measured":{"height":70,"width":160},"position":{"x":290,"y":390},"style":{"height":70,"width":160},"type":"shape"},{"data":{"color":"#ffffff","fill":"#ef4444","fontSize":"13px","text":"Fix Issues","type":"rectangle"},"id":"fc-fix","measured":{"height":56,"width":120},"position":{"x":520,"y":395},"style":{"height":56,"width":120},"type":"shape"},{"data":{"color":"#ffffff","fill":"#10b981","fontSize":"14px","fontWeight":"bold","text":"Deploy","type":"round-rectangle"},"id":"fc-end","measured":{"height":50,"width":120},"position":{"x":300,"y":530},"style":{"height":50,"width":120},"type":"shape"}],"title":"Untitled Diagram"}', '2026-07-13 12:11:53.301166+00') ON CONFLICT DO NOTHING;
INSERT INTO public.flow_diagrams (flow_id, diagram, updated_at) VALUES ('a0384751-f793-4315-9602-3043da6cdd32', '{"edges":[],"nodes":[],"title":"Untitled Diagram"}', '2026-07-13 12:12:24.723755+00') ON CONFLICT DO NOTHING;

-- invite_tokens
INSERT INTO public.invite_tokens (id, token, org_id, email, role, invited_by, expires_at, used_at, created_at) VALUES ('6919665c-d9f3-4d95-994e-92262b14f237', '4c4a4899-c2d6-4d6e-95b8-26b541581004', 'f2ef71f0-aaa1-454a-87c5-2614382896b3', 'khaleshubham-cmpn@atharvacoe.ac.in', 'member', '1de56956-a6ea-4fbe-843b-2125339399ed', '2026-07-05 09:43:53.630104+00', NULL, '2026-06-28 09:43:53.630104+00') ON CONFLICT DO NOTHING;
INSERT INTO public.invite_tokens (id, token, org_id, email, role, invited_by, expires_at, used_at, created_at) VALUES ('936da047-2dd5-42b4-b76c-b7ece0b7b108', '4bd2df12-485c-4982-aba0-e7fbea56d70b', 'f2ef71f0-aaa1-454a-87c5-2614382896b3', 'khaleshubham-cmpn@atharvacoe.ac.in', 'member', '1de56956-a6ea-4fbe-843b-2125339399ed', '2026-07-05 09:56:04.658174+00', NULL, '2026-06-28 09:56:04.658174+00') ON CONFLICT DO NOTHING;
INSERT INTO public.invite_tokens (id, token, org_id, email, role, invited_by, expires_at, used_at, created_at) VALUES ('ad44d714-e104-40b5-beaf-1b95e515a221', 'b5534af4-3b4b-4431-b4a4-9e3983223644', 'f2ef71f0-aaa1-454a-87c5-2614382896b3', 'khaleshubham-cmpn@atharvacoe.ac.in', 'member', '1de56956-a6ea-4fbe-843b-2125339399ed', '2026-07-05 10:00:12.87035+00', NULL, '2026-06-28 10:00:12.87035+00') ON CONFLICT DO NOTHING;
INSERT INTO public.invite_tokens (id, token, org_id, email, role, invited_by, expires_at, used_at, created_at) VALUES ('0733edb7-11e3-4f9f-b147-ddad58e40431', '81c1c94a-1eff-4c8c-bc90-9b518e3f37d0', 'f2ef71f0-aaa1-454a-87c5-2614382896b3', 'khaleshubham-cmpn@atharvacoe.ac.in', 'member', '1de56956-a6ea-4fbe-843b-2125339399ed', '2026-07-05 10:28:58.134456+00', '2026-06-28 10:39:16.150853+00', '2026-06-28 10:28:58.134456+00') ON CONFLICT DO NOTHING;

-- linked_resources
INSERT INTO public.linked_resources (id, board_id, label, url, created_by, created_at, updated_at) VALUES ('0a8c55da-db47-4adb-9240-c2c582614123', '346ff099-6f51-4c95-a951-1f3e7b755f88', 'design system', 'https://www.figma.com/file/example/design-system', '1de56956-a6ea-4fbe-843b-2125339399ed', '2026-07-13 07:04:18.676659+00', '2026-07-13 07:04:18.676659+00') ON CONFLICT DO NOTHING;
INSERT INTO public.linked_resources (id, board_id, label, url, created_by, created_at, updated_at) VALUES ('ea3840bc-01fd-4046-8cce-b2158b285daa', '346ff099-6f51-4c95-a951-1f3e7b755f88', 'github repo', 'https://github.com/example-org/product-roadmap', '1de56956-a6ea-4fbe-843b-2125339399ed', '2026-07-13 07:04:18.676659+00', '2026-07-13 07:04:18.676659+00') ON CONFLICT DO NOTHING;

-- organization_members
INSERT INTO public.organization_members (organization_id, user_id, role, status, invited_by, joined_at) VALUES ('f2ef71f0-aaa1-454a-87c5-2614382896b3', '1de56956-a6ea-4fbe-843b-2125339399ed', 'owner', 'active', NULL, '2026-06-26 13:30:54.630907+00') ON CONFLICT DO NOTHING;
INSERT INTO public.organization_members (organization_id, user_id, role, status, invited_by, joined_at) VALUES ('f2ef71f0-aaa1-454a-87c5-2614382896b3', 'ce313674-468e-4422-accd-ce729bef8717', 'member', 'active', '1de56956-a6ea-4fbe-843b-2125339399ed', '2026-06-28 10:39:13.343234+00') ON CONFLICT DO NOTHING;

-- organizations
INSERT INTO public.organizations (id, name, description, owner_id, created_at, updated_at) VALUES ('f2ef71f0-aaa1-454a-87c5-2614382896b3', 'SyncSpace', '', '1de56956-a6ea-4fbe-843b-2125339399ed', '2026-06-26 13:30:54.630907+00', '2026-06-26 13:30:54.630907+00') ON CONFLICT DO NOTHING;

-- tasks
INSERT INTO public.tasks (id, board_id, title, description, stage, priority, assignee_id, created_by, position, due_date, created_at, updated_at, start_date, tags, time_estimate, reference_link, flow_diagram_link, subtasks, attachments) VALUES ('06b78371-0752-4f48-bc93-2769de30f4cc', '346ff099-6f51-4c95-a951-1f3e7b755f88', 'Implement login page UI', 'Design and build the login page with email/password fields, validation, and OAuth buttons. Follow the Figma design spec.', 'Planning', 'medium', NULL, '1de56956-a6ea-4fbe-843b-2125339399ed', 0, NULL, '2026-07-11 09:49:55.233651+00', '2026-07-11 09:49:55.233651+00', NULL, '["Design","Frontend"]', '', '', '', '[]', '[]') ON CONFLICT DO NOTHING;
INSERT INTO public.tasks (id, board_id, title, description, stage, priority, assignee_id, created_by, position, due_date, created_at, updated_at, start_date, tags, time_estimate, reference_link, flow_diagram_link, subtasks, attachments) VALUES ('1081ef59-f46f-42bb-93d5-eb2c837c156b', '346ff099-6f51-4c95-a951-1f3e7b755f88', 'Design onboarding flow screens', 'Create Figma mockups for 5-step onboarding. Include mobile and desktop breakpoints.', 'Design', 'medium', NULL, '1de56956-a6ea-4fbe-843b-2125339399ed', 0, NULL, '2026-07-11 10:15:58.35986+00', '2026-07-11 10:15:58.35986+00', NULL, '["Frontend","Design"]', '', '', '', '[]', '[]') ON CONFLICT DO NOTHING;

-- user_notification_prefs
INSERT INTO public.user_notification_prefs (user_id, comments, invites, product_updates, updated_at) VALUES ('1de56956-a6ea-4fbe-843b-2125339399ed', TRUE, TRUE, TRUE, '2026-07-19 13:04:25.802207+00') ON CONFLICT DO NOTHING;

-- users
INSERT INTO public.users (id, name, email, bio, avatar_url, password_hash, created_at, updated_at) VALUES ('ce313674-468e-4422-accd-ce729bef8717', 'Aniket Sharma', 'khaleshubham-cmpn@atharvacoe.ac.in', '', '', '$2a$10$/RwJQF40dUrGJwcgh5ZePOZ0DtlAlx/qDHJ0BVZGAG16cTsxlqAf6', '2026-06-28 10:39:13.343234+00', '2026-06-28 10:39:13.343234+00') ON CONFLICT DO NOTHING;
INSERT INTO public.users (id, name, email, bio, avatar_url, password_hash, created_at, updated_at) VALUES ('1de56956-a6ea-4fbe-843b-2125339399ed', 'Shubham', 'shubhamkhalesk@gmail.com', '', 'https://res.cloudinary.com/dkgee8xdn/image/upload/v1783774751/syncspace/avatars/1de56956-a6ea-4fbe-843b-2125339399ed.jpg', '$2a$10$d5sHcmya4r82Dt4KsdRXGef7CvP5zoUrTziADwz9BXPIS4DkcFPy6', '2026-06-26 13:30:41.038364+00', '2026-07-11 12:59:17.987162+00') ON CONFLICT DO NOTHING;

