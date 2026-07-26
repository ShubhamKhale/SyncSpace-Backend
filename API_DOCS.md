# SyncSpace Backend — API Reference

> Base URL: `http://localhost:8068`  
> All `/api/*` responses use the envelope `{ "success": true, "data": ... }` or `{ "success": false, "error": "..." }`.  
> **Encrypted routes** (`POST /api/boards`, `GET /api/boards`) send/receive `{ "data": "<base64-AES-GCM>" }` on the wire — the payload documented below is the **plaintext** inside.

---

## Authentication

| Method | Path | Auth | Encrypted |
|--------|------|------|-----------|
| POST | `/api/auth/register` | Public | No |
| POST | `/api/auth/signin` | Public | No |
| POST | `/api/auth/logout` | Bearer JWT | No |

---

### POST `/api/auth/register`

**Purpose:** Create a new account. If `invite_token` is provided the user is auto-joined to the inviting org.

**Request body:**
```json
{
  "name":         "Alice",
  "email":        "alice@example.com",
  "password":     "secret123",
  "invite_token": "optional-uuid-from-invite-link"
}
```

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 201 | Success | `{ "token": "jwt...", "session_key": "hex...", "user": { "id":"...", "name":"...", "email":"...", "avatarUrl":null, "role":null, "orgId":null, "hasOrg":false } }` |
| 400 | Validation error / password < 8 chars / bad email | `{ "error": "..." }` |
| 409 | Email already registered | `{ "error": "email already in use" }` |
| 500 | DB error | `{ "error": "internal server error" }` |

**Backend queries:**
```sql
-- check uniqueness
SELECT id FROM public.users WHERE email = $1

-- insert user
INSERT INTO public.users (id, name, email, password_hash, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6)

-- if invite_token provided — validate token
SELECT id, token, org_id, email, role, invited_by, expires_at, used_at, created_at
FROM public.invite_tokens WHERE token = $1

-- join org
INSERT INTO public.organization_members
    (organization_id, user_id, role, status, invited_by, joined_at)
VALUES ($1,$2,$3,'active',$4,$5)

-- mark token used
UPDATE public.invite_tokens SET used_at = $2 WHERE id = $1

-- resolve AuthUser
SELECT u.id, u.name, u.email, u.avatar_url,
       CASE WHEN o.owner_id = u.id THEN 'owner' ELSE om.role END AS role,
       o.id AS org_id
FROM   public.users u
LEFT JOIN public.organization_members om ON om.user_id = u.id AND om.status = 'active'
LEFT JOIN public.organizations o ON o.id = om.organization_id
WHERE  u.id = $1
LIMIT  1
```

---

### POST `/api/auth/signin`

**Purpose:** Authenticate with email + password, receive JWT + session key.

**Request body:**
```json
{
  "email":    "alice@example.com",
  "password": "secret123"
}
```

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `{ "token": "jwt...", "session_key": "hex...", "user": { AuthUser } }` |
| 400 | Missing/invalid fields | `{ "error": "..." }` |
| 401 | Wrong email or password | `{ "error": "invalid email or password" }` |

**Backend queries:**
```sql
SELECT id, name, email, password_hash, avatar_url, created_at, updated_at
FROM public.users WHERE email = $1

-- then same AuthUser LEFT JOIN query as register
```

---

### POST `/api/auth/logout`

**Purpose:** Revoke the server-side session key (in-memory store). JWT itself remains valid until expiry.

**Headers:** `Authorization: Bearer <token>`

**Request body:** none

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `{ "message": "logged out" }` |
| 401 | Missing/invalid JWT | `{ "error": "authentication required" }` |

**Backend:** In-memory session key delete — no SQL.

---

## User

| Method | Path | Auth |
|--------|------|------|
| GET | `/api/user/me` | Bearer JWT |
| PATCH | `/api/user/profile` | Bearer JWT |
| PATCH | `/api/user/avatar` | Bearer JWT |

---

### GET `/api/user/me`

**Purpose:** Return the authenticated user's profile including org membership.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `{ "id":"...", "name":"...", "email":"...", "avatarUrl":"...\|null", "role":"owner\|admin\|member\|viewer\|null", "orgId":"...\|null", "hasOrg":true\|false }` |
| 401 | Bad JWT | `{ "error": "..." }` |
| 404 | User deleted | `{ "error": "user not found" }` |

**Backend query:**
```sql
SELECT u.id, u.name, u.email, u.avatar_url,
       CASE WHEN o.owner_id = u.id THEN 'owner' ELSE om.role END AS role,
       o.id AS org_id
FROM   public.users u
LEFT JOIN public.organization_members om ON om.user_id = u.id AND om.status = 'active'
LEFT JOIN public.organizations o ON o.id = om.organization_id
WHERE  u.id = $1
LIMIT  1
```

---

### PATCH `/api/user/profile`

**Purpose:** Update name and/or email. Send only the fields to change.

**Request body:**
```json
{
  "name":  "Alice B",
  "email": "new@example.com"
}
```

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `{ AuthUser }` |
| 400 | No fields / invalid email | `{ "error": "..." }` |
| 401 | Bad JWT | `{ "error": "..." }` |
| 409 | Email already taken | `{ "error": "email already in use" }` |

**Backend query:**
```sql
UPDATE public.users SET name=$2, email=$3, updated_at=$4 WHERE id=$1
-- only updates supplied fields (nil fields skipped in service layer)
```

---

### PATCH `/api/user/avatar`

**Purpose:** Store an avatar URL (client uploads to object storage first, then passes URL here).

**Request body:**
```json
{ "avatar_url": "https://cdn.example.com/avatars/alice.jpg" }
```

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `{ AuthUser }` |
| 400 | Missing / invalid URL | `{ "error": "..." }` |
| 401 | Bad JWT | `{ "error": "..." }` |

**Backend query:**
```sql
UPDATE public.users SET avatar_url=$2, updated_at=$3 WHERE id=$1
```

---

## Organization

| Method | Path | Auth |
|--------|------|------|
| POST | `/api/organization` | Bearer JWT |
| GET | `/api/organization` | Bearer JWT |
| PATCH | `/api/organization/:id` | Bearer JWT |
| POST | `/api/organization/invite` | Bearer JWT |
| GET | `/api/organization/invite/verify` | Public |
| GET | `/api/organization/members` | Bearer JWT |
| POST | `/api/organization/members/invite` | Bearer JWT |
| PATCH | `/api/organization/members/:id/role` | Bearer JWT |
| DELETE | `/api/organization/members/:id` | Bearer JWT |

---

### POST `/api/organization`

**Purpose:** Create a new organization. Caller becomes owner and is auto-added as `owner` member.

**Request body:**
```json
{ "name": "Acme Corp" }
```

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 201 | Created | `{ "id": "...", "name": "Acme Corp" }` |
| 400 | Missing name / name > 255 chars | `{ "error": "..." }` |
| 409 | Caller already owns/belongs to an org | `{ "error": "..." }` |

**Backend queries:**
```sql
INSERT INTO public.organizations (id, name, description, owner_id, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6)

INSERT INTO public.organization_members
    (organization_id, user_id, role, status, invited_by, joined_at)
VALUES ($1,$2,'owner','active',NULL,$3)
```

---

### GET `/api/organization`

**Purpose:** List all organizations the caller belongs to with their role.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `[ { "id":"...", "name":"...", "description":"...", "owner_id":"...", "created_at":"...", "updated_at":"...", "role":"owner" } ]` |

**Backend query:**
```sql
SELECT o.id, o.name, o.description, o.owner_id, o.created_at, o.updated_at,
       CASE WHEN o.owner_id = $1 THEN 'owner' ELSE om.role END AS role
FROM   public.organizations o
JOIN   public.organization_members om ON om.organization_id = o.id AND om.user_id = $1
ORDER  BY o.created_at ASC
```

---

### PATCH `/api/organization/:id`

**Purpose:** Update org name and/or description. Caller must be owner.

**Request body:**
```json
{
  "name":        "Acme Ltd",
  "description": "Updated description"
}
```

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Updated | `{ Organization }` |
| 400 | No fields provided | `{ "error": "provide at least one field to update" }` |
| 403 | Caller not owner | `{ "error": "..." }` |
| 404 | Org not found | `{ "error": "organization not found" }` |

**Backend query:**
```sql
UPDATE public.organizations SET name=$2, description=$3, updated_at=$4 WHERE id=$1
```

---

### POST `/api/organization/invite`

**Purpose:** Generate a one-time invite-link token for an email address. Token valid 7 days.

**Request body:**
```json
{
  "email": "bob@example.com",
  "role":  "member"
}
```

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 201 | Token created | `{ "inviteId": "...", "token": "uuid" }` |
| 400 | Bad role / missing fields | `{ "error": "..." }` |
| 403 | Caller not owner or admin / no org | `{ "error": "..." }` |

**Backend query:**
```sql
INSERT INTO public.invite_tokens
    (id, token, org_id, email, role, invited_by, expires_at, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
```

---

### GET `/api/organization/invite/verify`

**Purpose:** Validate an invite token and return org details so the signup page can show org name. Public — no JWT needed.

**Query params:** `?token=<uuid>`

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Valid token | `{ "token":"...", "org_id":"...", "org_name":"...", "email":"...", "role":"..." }` |
| 400 | Missing token | `{ "error": "token query parameter is required" }` |
| 404 | Token not found / already used / expired | `{ "error": "..." }` |

**Backend query:**
```sql
SELECT it.id, it.token, it.org_id, it.email, it.role,
       it.invited_by, it.expires_at, it.used_at, it.created_at,
       o.name AS org_name
FROM   public.invite_tokens it
JOIN   public.organizations o ON o.id = it.org_id
WHERE  it.token = $1
  AND  it.used_at IS NULL
  AND  it.expires_at > NOW()
```

---

### GET `/api/organization/members`

**Purpose:** List all members of the caller's org with public profile info.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `[ { "user_id":"...", "name":"...", "email":"...", "avatar_url":"...", "role":"...", "status":"active", "invited_by":"...", "joined_at":"..." } ]` |
| 403 | No org | `{ "error": "you must belong to an organization" }` |

**Backend query:**
```sql
SELECT om.user_id, u.name, u.email, u.avatar_url,
       om.role, om.status, COALESCE(om.invited_by,''), om.joined_at
FROM   public.organization_members om
JOIN   public.users u ON u.id = om.user_id
WHERE  om.organization_id = $1
ORDER  BY om.joined_at ASC
```

---

### POST `/api/organization/members/invite`

**Purpose:** Directly add an existing user to an org by email. Role must be `admin|member|viewer`.

**Request body:**
```json
{
  "org_id": "org-uuid",
  "email":  "bob@example.com",
  "role":   "member"
}
```

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 201 | Invited | `{ OrgMemberDetail }` |
| 400 | Missing fields / invalid role | `{ "error": "..." }` |
| 403 | Caller lacks permission | `{ "error": "..." }` |
| 404 | User email not found | `{ "error": "user not found" }` |
| 409 | Already a member | `{ "error": "user is already a member of this organization" }` |

---

### PATCH `/api/organization/members/:id/role`

**Purpose:** Change a member's role. Caller must be owner or admin. Cannot set role to `owner`.

**Request body:**
```json
{ "role": "admin" }
```

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Updated | `{ "user_id": "...", "role": "admin" }` |
| 400 | Invalid role | `{ "error": "..." }` |
| 403 | Caller not owner/admin / no org | `{ "error": "..." }` |
| 404 | Member not found | `{ "error": "member not found" }` |

**Backend query:**
```sql
UPDATE public.organization_members SET role=$3 WHERE organization_id=$1 AND user_id=$2
```

---

### DELETE `/api/organization/members/:id`

**Purpose:** Remove a member from the org. Caller must be owner or admin.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Removed | `{ "removed": true }` |
| 403 | No permission / no org | `{ "error": "..." }` |
| 404 | Member not found | `{ "error": "member not found" }` |

**Backend query:**
```sql
DELETE FROM public.organization_members WHERE organization_id=$1 AND user_id=$2
```

---

## Boards

| Method | Path | Auth | Encrypted |
|--------|------|------|-----------|
| POST | `/api/boards` | Bearer JWT | **Yes** |
| GET | `/api/boards` | Bearer JWT | **Yes** |
| GET | `/api/boards/recent` | Bearer JWT | No |
| GET | `/api/boards/:id/health` | Bearer JWT | No |
| GET | `/api/boards/:id/activity` | Bearer JWT | No |
| GET | `/api/boards/:id/linked-resources` | Bearer JWT | No |
| PUT | `/api/boards/:id/linked-resources` | Bearer JWT | No |

> **Encryption note:** `POST /api/boards` and `GET /api/boards` use AES-256-GCM. The wire body is `{ "data": "<base64>" }`. The decrypted plaintext is documented below.

---

### POST `/api/boards` *(encrypted)*

**Purpose:** Create a new board scoped to the caller's org.

**Plaintext request body:**
```json
{
  "title":       "Q3 Sprint",
  "description": "Optional description"
}
```

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 201 | Created | `{ Board }` (encrypted on wire) |
| 400 | Missing title / validation | `{ "error": "..." }` |
| 403 | No org | `{ "error": "you must belong to an organization to create boards" }` |

**Backend queries:**
```sql
-- derive position
SELECT COALESCE(MAX(position)+1, 0) FROM public.tasks WHERE board_id=$1 AND stage=$2

-- insert board
INSERT INTO public.boards (id, title, description, owner_id, org_id, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)
```

---

### GET `/api/boards` *(encrypted)*

**Purpose:** List all boards belonging to the caller's org, newest first.

**Plaintext response `data`:**
```json
[
  {
    "id": "...", "title": "Q3 Sprint", "description": "...",
    "owner_id": "...", "org_id": "...",
    "created_at": "...", "updated_at": "..."
  }
]
```

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | array of Board (encrypted) |
| 403 | No org | `{ "error": "you must belong to an organization to view boards" }` |

**Backend query:**
```sql
SELECT id, title, description, owner_id, org_id, created_at, updated_at
FROM   public.boards
WHERE  org_id = $1
ORDER  BY created_at DESC
```

---

### GET `/api/boards/recent`

**Purpose:** Return the most recently updated boards for the org. Used by dashboard.

**Query params:** `?limit=5` (default 5)

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `[ { Board } ]` (empty array if no org) |

**Backend query:**
```sql
SELECT id, title, description, owner_id, org_id, created_at, updated_at
FROM   public.boards WHERE org_id = $1
ORDER  BY updated_at DESC LIMIT $2
```

---

### GET `/api/boards/:id/health`

**Purpose:** Compute a risk-based health summary from the board's tasks.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `{ "status":"healthy\|warning\|critical", "overdueTasks":2, "bottlenecks":0, "atRisk":1 }` |
| 403 | No org | `{ "error": "you must belong to an organization" }` |
| 404 | Board not found / not in org | `{ "error": "board not found" }` |

**Status rules:** `critical` if `overdueTasks > 5`; `warning` if any signal > 0; else `healthy`. `bottlenecks` always 0 (stage-change timestamps not tracked yet).

**Backend queries:**
```sql
-- verify board belongs to org
SELECT id,... FROM public.boards WHERE id=$1 AND org_id=$2

-- compute risk counts
SELECT
    COUNT(*) FILTER (
        WHERE due_date IS NOT NULL AND due_date < NOW() AND stage != 'done'
    )::int AS overdue,
    COUNT(*) FILTER (
        WHERE priority='high' AND due_date IS NOT NULL
          AND due_date >= NOW() AND due_date <= NOW() + INTERVAL '2 days'
          AND stage != 'done'
    )::int AS at_risk
FROM public.tasks WHERE board_id = $1
```

---

### GET `/api/boards/:id/activity`

**Purpose:** Recent activity for a single board (board events + task events on that board).

**Query params:** `?limit=10` (default 10, max 50)

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `[ { "id":"...", "actor":"Alice", "action":"added a new task", "target":"Fix login bug", "createdAt":"..." } ]` |
| 403 | No org | `{ "error": "you must belong to an organization" }` |
| 404 | Board not found / not in org | `{ "error": "board not found" }` |

**Action humanisation (done in SQL CASE):**

| entity_type | action | Human string |
|-------------|--------|--------------|
| task | created | added a new task |
| task | updated | updated a task |
| task | moved | moved a task |
| task | assigned | assigned a task |
| task | deleted | deleted a task |
| board | created | created the board |
| board | updated | updated the board |

**Backend query:**
```sql
SELECT
    al.id,
    COALESCE(u.name,'') AS actor,
    CASE
        WHEN al.entity_type='task'  AND al.action='created'  THEN 'added a new task'
        WHEN al.entity_type='task'  AND al.action='updated'  THEN 'updated a task'
        WHEN al.entity_type='task'  AND al.action='moved'    THEN 'moved a task'
        WHEN al.entity_type='task'  AND al.action='assigned' THEN 'assigned a task'
        WHEN al.entity_type='task'  AND al.action='deleted'  THEN 'deleted a task'
        WHEN al.entity_type='board' AND al.action='created'  THEN 'created the board'
        WHEN al.entity_type='board' AND al.action='updated'  THEN 'updated the board'
        ELSE al.action || ' ' || al.entity_type
    END AS action,
    COALESCE(t_ref.title, b_direct.title, '') AS target,
    al.created_at
FROM   public.activity_logs al
JOIN   public.users u ON u.id = al.actor_id
LEFT JOIN public.boards b_direct ON b_direct.id = al.entity_id AND al.entity_type='board'
LEFT JOIN public.tasks  t_ref    ON t_ref.id = al.entity_id    AND al.entity_type='task'
WHERE  b_direct.id = $1 OR t_ref.board_id = $1
ORDER  BY al.created_at DESC LIMIT $2
```

---

### GET `/api/boards/:id/linked-resources`

**Purpose:** Return all external links (Figma, Notion, docs, etc.) attached to a board.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `[ { "id":"...", "board_id":"...", "label":"Figma", "url":"https://...", "created_by":"...", "created_at":"...", "updated_at":"..." } ]` |

**Backend query:**
```sql
SELECT id, board_id, label, url, created_by, created_at, updated_at
FROM   public.linked_resources WHERE board_id=$1 ORDER BY created_at ASC
```

---

### PUT `/api/boards/:id/linked-resources`

**Purpose:** Atomically replace the full set of linked resources for a board. Send `[]` to clear all.

**Request body:**
```json
{
  "resources": [
    { "label": "Figma",  "url": "https://figma.com/..." },
    { "label": "Notion", "url": "https://notion.so/..." }
  ]
}
```

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Replaced | `[ { LinkedResource } ]` |
| 400 | Missing `resources` key | `{ "error": "..." }` |

**Backend queries (single transaction):**
```sql
DELETE FROM public.linked_resources WHERE board_id=$1

INSERT INTO public.linked_resources (id, board_id, label, url, created_by, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)   -- repeated for each item
```

---

## Tasks

| Method | Path | Auth |
|--------|------|------|
| GET | `/api/boards/:id/tasks` | Bearer JWT |
| POST | `/api/boards/:id/tasks` | Bearer JWT |
| PATCH | `/api/tasks/:id` | Bearer JWT |
| PATCH | `/api/tasks/:id/stage` | Bearer JWT |
| GET | `/api/tasks/upcoming` | Bearer JWT |

---

### GET `/api/boards/:id/tasks`

**Purpose:** List all tasks on a board, ordered by stage then position (Kanban-ready).

**Query params (all optional):**

| Param | Values |
|-------|--------|
| `stage` | `todo \| in_progress \| done` |
| `priority` | `low \| medium \| high` |
| `assignee_id` | user UUID |

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `[ { Task } ]` |
| 400 | Invalid stage or priority | `{ "error": "..." }` |

**Backend query:**
```sql
SELECT id, board_id, title, description, stage, priority,
       COALESCE(assignee_id,''), created_by, position, due_date, created_at, updated_at
FROM   public.tasks
WHERE  board_id=$1
  AND ($2::text IS NULL OR stage=$2)
  AND ($3::text IS NULL OR priority=$3)
  AND ($4::text IS NULL OR assignee_id=$4)
ORDER BY stage, position ASC
```

---

### POST `/api/boards/:id/tasks`

**Purpose:** Create a new task on a board. Auto-assigns position to bottom of the stage column.

**Request body:**
```json
{
  "title":       "Fix login bug",
  "description": "Optional",
  "stage":       "todo",
  "priority":    "high",
  "assignee_id": "user-uuid",
  "due_date":    "2026-07-01T09:00:00Z"
}
```

`stage` defaults to `todo`, `priority` defaults to `medium`, all others optional.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 201 | Created | `{ Task }` |
| 400 | Missing title / bad due_date format / invalid stage+priority | `{ "error": "..." }` |

**Backend queries:**
```sql
-- derive bottom position
SELECT COALESCE(MAX(position)+1, 0) FROM public.tasks WHERE board_id=$1 AND stage=$2

-- insert
INSERT INTO public.tasks
    (id, board_id, title, description, stage, priority,
     assignee_id, created_by, position, due_date, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9,$10,$11,$12)
```

---

### PATCH `/api/tasks/:id`

**Purpose:** Partial update — only fields present in the body are changed.

**Request body (all fields optional):**
```json
{
  "title":          "Updated title",
  "description":    "Updated desc",
  "priority":       "low",
  "assignee_id":    "user-uuid",
  "due_date":       "2026-08-01T00:00:00Z",
  "clear_due_date": false
}
```

Set `clear_due_date: true` to remove `due_date` without supplying a new value.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Updated | `{ Task }` |
| 400 | Invalid field values | `{ "error": "..." }` |
| 404 | Task not found | `{ "error": "task not found" }` |

**Backend query:**
```sql
UPDATE public.tasks
SET    title=$2, description=$3, priority=$4,
       assignee_id=NULLIF($5,''), due_date=$6, updated_at=$7
WHERE  id=$1
```

---

### PATCH `/api/tasks/:id/stage`

**Purpose:** Drag-and-drop: move task to a new stage at a specific position. Keeps sibling positions contiguous via transaction.

**Query params:** `?board_id=<board-uuid>` (required)

**Request body:**
```json
{ "stage": "in_progress", "position": 2 }
```

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Moved | `{ Task }` |
| 400 | Missing board_id / invalid stage / position < 0 | `{ "error": "..." }` |
| 404 | Task not found | `{ "error": "task not found" }` |

**Backend queries (transaction):**
```sql
-- 1. fetch current state
SELECT stage, position FROM public.tasks WHERE id=$1 AND board_id=$2

-- 2. close gap in source column
UPDATE public.tasks SET position=position-1
WHERE board_id=$1 AND stage=$2 AND position>$3 AND id!=$4

-- 3. open slot in destination column
UPDATE public.tasks SET position=position+1
WHERE board_id=$1 AND stage=$2 AND position>=$3 AND id!=$4

-- 4. place task
UPDATE public.tasks SET stage=$2, position=$3, updated_at=$4 WHERE id=$1
```

---

### GET `/api/tasks/upcoming`

**Purpose:** Non-done tasks due within the next 7 days assigned to or created by the caller.

**Query params:** `?limit=10` (default 10)

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `[ { Task } ]` ordered by `due_date ASC` |

**Backend query (UNION ALL avoids OR on two columns):**
```sql
SELECT ... FROM (
    SELECT ... FROM public.tasks
    WHERE  assignee_id=$1 AND stage IN ('todo','in_progress')
      AND  due_date BETWEEN NOW() AND NOW() + INTERVAL '7 days'
    UNION ALL
    SELECT ... FROM public.tasks
    WHERE  created_by=$1 AND assignee_id IS NULL
      AND  stage IN ('todo','in_progress')
      AND  due_date BETWEEN NOW() AND NOW() + INTERVAL '7 days'
) sub
ORDER BY due_date ASC LIMIT $2
```

---

## Dashboard

| Method | Path | Auth |
|--------|------|------|
| GET | `/api/dashboard/stats` | Bearer JWT |

---

### GET `/api/dashboard/stats`

**Purpose:** Aggregated counts for org boards and tasks — used for the dashboard overview cards.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success (with org) | `{ "board_count":3, "task_todo":12, "task_in_progress":5, "task_done":20, "task_due_soon":2 }` |
| 200 | No org yet | `{ "board_count":0, "task_todo":0, "task_in_progress":0, "task_done":0, "task_due_soon":0 }` |

**Backend query:**
```sql
WITH org_boards AS (SELECT id FROM public.boards WHERE org_id=$1)
SELECT
    (SELECT COUNT(*)::int FROM org_boards)                              AS board_count,
    COUNT(*) FILTER (WHERE stage='todo')::int                           AS task_todo,
    COUNT(*) FILTER (WHERE stage='in_progress')::int                    AS task_in_progress,
    COUNT(*) FILTER (WHERE stage='done')::int                           AS task_done,
    COUNT(*) FILTER (
        WHERE stage!='done' AND due_date IS NOT NULL
          AND due_date BETWEEN NOW() AND NOW() + INTERVAL '7 days'
    )::int                                                              AS task_due_soon
FROM public.tasks WHERE board_id IN (SELECT id FROM org_boards)
```

---

## Activity Logs

| Method | Path | Auth |
|--------|------|------|
| GET | `/api/activity-logs` | Bearer JWT |

---

### GET `/api/activity-logs`

**Purpose:** Paginated activity feed with optional board/user filters.

**Query params:**

| Param | Default | Notes |
|-------|---------|-------|
| `board_id` | — | filter to board + its tasks |
| `user_id` | caller | filter by actor |
| `limit` | 20 | max 100 |
| `offset` | 0 | |

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `{ "logs": [ { ActivityLogDetail } ], "total": 42 }` |

Each `ActivityLogDetail`:
```json
{
  "id":"...", "entity_type":"task", "entity_id":"...",
  "actor_id":"...", "action":"created", "metadata":{...},
  "created_at":"...", "actor_name":"Alice"
}
```

**Backend query:**
```sql
SELECT al.id, al.entity_type, al.entity_id, al.actor_id, al.action,
       al.metadata, al.created_at, COALESCE(u.name,'') AS actor_name,
       COUNT(*) OVER() AS total_count
FROM   public.activity_logs al
JOIN   public.users u ON u.id = al.actor_id
LEFT JOIN public.boards b_direct ON b_direct.id=al.entity_id AND al.entity_type='board'
LEFT JOIN public.tasks  t_ref    ON t_ref.id=al.entity_id    AND al.entity_type='task'
LEFT JOIN public.boards b_task   ON b_task.id=t_ref.board_id
WHERE ($1::text IS NULL OR COALESCE(b_direct.org_id, b_task.org_id)=$1)
  AND ($2::text IS NULL OR COALESCE(b_direct.id, b_task.id)=$2)
  AND ($3::text IS NULL OR al.actor_id=$3)
ORDER BY al.created_at DESC LIMIT $4 OFFSET $5
```

---

## Notifications

| Method | Path | Auth |
|--------|------|------|
| GET | `/api/notifications` | Bearer JWT |
| PATCH | `/api/notifications/mark-all-read` | Bearer JWT |
| PATCH | `/api/notifications/:id/read` | Bearer JWT |

---

### GET `/api/notifications`

**Purpose:** Paginated notification list with global unread count.

**Query params:** `?limit=20&offset=0`

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `{ "notifications": [ { Notification } ], "unread_count": 3, "total": 25 }` |

Each `Notification`:
```json
{
  "id":"...", "user_id":"...", "title":"You were invited",
  "body":"...", "is_read":false,
  "entity_type":"org", "entity_id":"...", "created_at":"..."
}
```

**Backend query:**
```sql
SELECT id, user_id, title, body, is_read,
       COALESCE(entity_type,''), COALESCE(entity_id,''), created_at,
       COUNT(*) FILTER (WHERE NOT is_read) OVER() AS unread_count,
       COUNT(*)                            OVER() AS total_count
FROM   public.notifications
WHERE  user_id=$1
ORDER  BY created_at DESC LIMIT $2 OFFSET $3
```

---

### PATCH `/api/notifications/mark-all-read`

**Purpose:** Mark every unread notification for the caller as read in one statement.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `{ "marked_read": 5 }` |

**Backend query:**
```sql
UPDATE public.notifications SET is_read=TRUE WHERE user_id=$1 AND is_read=FALSE
```

---

### PATCH `/api/notifications/:id/read`

**Purpose:** Mark a single notification as read. Enforces ownership via WHERE clause.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Marked | `{ "id":"...", "is_read":true }` |
| 404 | Not found or wrong user | `{ "error": "notification not found" }` |

**Backend query:**
```sql
UPDATE public.notifications SET is_read=TRUE WHERE id=$1 AND user_id=$2
```

---

## Analytics

All analytics endpoints require Bearer JWT. `org_id` is injected by `OrgContext` middleware.

| Method | Path |
|--------|------|
| GET | `/api/analytics/task-completion` |
| GET | `/api/analytics/task-completion-trend` |
| GET | `/api/analytics/task-distribution` |
| GET | `/api/analytics/board-activity` |
| GET | `/api/analytics/team-contribution` |
| GET | `/api/analytics/dashboard` |

---

### GET `/api/analytics/task-completion`

**Purpose:** Daily created vs completed task counts for a time window (line/bar chart).

**Query params:**

| Param | Values | Default |
|-------|--------|---------|
| `period` | `week \| month \| quarter` | `week` |
| `board_id` | board UUID | all boards |

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `[ { "date":"2026-06-01", "created":3, "completed":1 } ]` |

**Backend query:**
```sql
-- generates a complete date series (no gaps) for the period
WITH date_series AS (
    SELECT generate_series($2::date, NOW()::date, '1 day')::date AS day
)
SELECT ds.day::text AS date,
       COUNT(t.id) FILTER (WHERE t.created_at::date = ds.day)::int AS created,
       COUNT(t.id) FILTER (WHERE t.updated_at::date = ds.day AND t.stage='done')::int AS completed
FROM   date_series ds
LEFT JOIN public.tasks t ON t.board_id IN (
    SELECT id FROM public.boards WHERE org_id=$1
) AND ($3::text IS NULL OR t.board_id=$3)
GROUP BY ds.day ORDER BY ds.day
```

---

### GET `/api/analytics/task-completion-trend`

**Purpose:** Monthly created + completed counts for the last 7 months. Used by dashboard line chart.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `[ { "month":"Jan", "created":12, "completed":10 } ]` |
| 403 | No org | `{ "error": "you must belong to an organization" }` |

**Backend query:**
```sql
WITH months AS (
    SELECT generate_series(
        date_trunc('month', NOW()) - INTERVAL '6 months',
        date_trunc('month', NOW()),
        '1 month'
    ) AS m
),
org_boards AS (SELECT id FROM public.boards WHERE org_id=$1)
SELECT TO_CHAR(months.m,'Mon') AS month,
       COUNT(t.id) FILTER (WHERE date_trunc('month',t.created_at)=months.m)::int AS created,
       COUNT(t.id) FILTER (WHERE date_trunc('month',t.updated_at)=months.m AND t.stage='done')::int AS completed
FROM months
LEFT JOIN public.tasks t ON t.board_id IN (SELECT id FROM org_boards)
GROUP BY months.m ORDER BY months.m
```

---

### GET `/api/analytics/task-distribution`

**Purpose:** Task counts grouped by stage as `{ name, value }` pairs for a pie/donut chart.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `[ { "name":"To Do", "value":8 }, { "name":"In Progress", "value":3 }, { "name":"Done", "value":15 } ]` |
| 403 | No org | `{ "error": "..." }` |

**Backend query:**
```sql
WITH org_boards AS (SELECT id FROM public.boards WHERE org_id=$1)
SELECT
    CASE stage WHEN 'todo' THEN 'To Do' WHEN 'in_progress' THEN 'In Progress' ELSE 'Done' END AS name,
    COUNT(*)::int AS value
FROM public.tasks WHERE board_id IN (SELECT id FROM org_boards)
GROUP BY stage ORDER BY CASE stage WHEN 'todo' THEN 1 WHEN 'in_progress' THEN 2 ELSE 3 END
```

---

### GET `/api/analytics/board-activity`

**Purpose:** Top 5 boards by edit/comment/share counts for a bar chart.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `[ { "name":"Sprint Board", "edits":12, "comments":0, "shares":0 } ]` |
| 403 | No org | `{ "error": "..." }` |

**Backend query:**
```sql
WITH org_boards AS (SELECT id, title FROM public.boards WHERE org_id=$1),
board_edits AS (
    SELECT entity_id AS board_id, COUNT(*) AS edits
    FROM public.activity_logs WHERE entity_type='board' AND action='updated'
      AND entity_id IN (SELECT id FROM org_boards)
    GROUP BY entity_id
),
task_edits AS (
    SELECT t.board_id, COUNT(*) AS edits
    FROM public.activity_logs al
    JOIN public.tasks t ON t.id=al.entity_id
    WHERE al.entity_type='task' AND t.board_id IN (SELECT id FROM org_boards)
    GROUP BY t.board_id
)
SELECT ob.title AS name,
       COALESCE(be.edits,0)+COALESCE(te.edits,0) AS edits,
       0 AS comments, 0 AS shares
FROM org_boards ob
LEFT JOIN board_edits be ON be.board_id=ob.id
LEFT JOIN task_edits  te ON te.board_id=ob.id
ORDER BY edits DESC LIMIT 5
```

---

### GET `/api/analytics/team-contribution`

**Purpose:** Radar chart data — per-member task counts across Planning / Development / Deployment phases for the top 3 contributors.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `{ "data": [ { "phase":"Planning", "Alice":5, "Bob":2 }, ... ], "members": ["Alice","Bob"] }` |
| 403 | No org | `{ "error": "..." }` |

**Backend query:**
```sql
WITH org_boards AS (SELECT id FROM public.boards WHERE org_id=$1),
top_members AS (
    SELECT u.id, u.name, COUNT(*) AS total
    FROM   public.tasks t
    JOIN   public.users u ON u.id=COALESCE(t.assignee_id, t.created_by)
    WHERE  t.board_id IN (SELECT id FROM org_boards)
    GROUP  BY u.id, u.name ORDER BY total DESC LIMIT 3
)
SELECT u.id, u.name,
       COUNT(*) FILTER (WHERE t.stage='todo')::int        AS planning,
       COUNT(*) FILTER (WHERE t.stage='in_progress')::int AS development,
       COUNT(*) FILTER (WHERE t.stage='done')::int        AS deployment
FROM   public.tasks t
JOIN   public.users u ON u.id=COALESCE(t.assignee_id, t.created_by)
WHERE  t.board_id IN (SELECT id FROM org_boards)
  AND  u.id IN (SELECT id FROM top_members)
GROUP  BY u.id, u.name
ORDER  BY COUNT(*) DESC
```

---

### GET `/api/analytics/dashboard`

**Purpose:** Single endpoint returning all 4 analytics datasets in one response. Used by the analytics dashboard page.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | See shape below |
| 403 | No org | `{ "error": "you must belong to an organization" }` |
| 500 | Query error | `{ "error": "failed to query ..." }` |

**Response shape:**
```json
{
  "taskCompletionTrend": [ { "month":"Jan", "created":12, "completed":10 } ],
  "taskDistribution":    [ { "name":"To Do", "value":8 } ],
  "boardActivity":       [ { "name":"Sprint", "edits":5, "comments":0, "shares":0 } ],
  "teamContribution": {
    "data":    [ { "phase":"Planning", "Alice":5, "Bob":2 } ],
    "members": ["Alice","Bob"]
  }
}
```

**Backend:** Calls four queries sequentially — same SQL as the four individual endpoints above.

---

## Flows

| Method | Path | Auth |
|--------|------|------|
| GET | `/api/flows/:id` | Bearer JWT |
| PUT | `/api/flows/:id` | Bearer JWT |
| GET | `/api/flows/:id/votes` | Bearer JWT |
| POST | `/api/flows/:id/votes` | Bearer JWT |

---

### GET `/api/flows/:id`

**Purpose:** Fetch a flow diagram including its JSONB node/edge data and current version.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `{ "id":"...", "board_id":"...", "title":"...", "data":{"nodes":[...],"edges":[...]}, "version":3, "last_modified_by":"...", "created_by":"...", "created_at":"...", "updated_at":"..." }` |
| 404 | Not found | `{ "error": "flow not found" }` |

**Backend query:**
```sql
SELECT id, board_id, title, data, version, last_modified_by, created_by, created_at, updated_at
FROM public.flows WHERE id=$1
```

---

### PUT `/api/flows/:id`

**Purpose:** Replace flow data with optimistic concurrency control. Send the version you last read; server rejects if it has changed.

**Request body:**
```json
{
  "data":    { "nodes": [...], "edges": [...] },
  "version": 3
}
```

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Saved | `{ Flow }` with `version` incremented |
| 400 | Missing `data` | `{ "error": "..." }` |
| 404 | Flow not found | `{ "error": "flow not found" }` |
| 409 | Version mismatch (concurrent edit) | `{ "error": "flow was modified by another client; re-fetch the latest version and retry" }` |

**Backend query:**
```sql
UPDATE public.flows
SET    data=$2, version=version+1, last_modified_by=$3, updated_at=$4
WHERE  id=$1 AND version=$5   -- optimistic lock: fails if version changed
```

---

### GET `/api/flows/:id/votes`

**Purpose:** Vote totals and the caller's own vote for a flow.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Success | `{ "up":5, "down":1, "my_vote":"up" }` |

`my_vote` is `""` when the caller has not voted.

**Backend query:**
```sql
SELECT
    COUNT(*) FILTER (WHERE vote_type='up')::int   AS up,
    COUNT(*) FILTER (WHERE vote_type='down')::int AS down,
    COALESCE(MAX(vote_type) FILTER (WHERE user_id=$2),'') AS my_vote
FROM public.flow_votes WHERE flow_id=$1
```

---

### POST `/api/flows/:id/votes`

**Purpose:** Cast or change the caller's vote on a flow. One vote per user per flow (upsert).

**Request body:**
```json
{ "vote_type": "up" }
```

`vote_type`: `up` or `down`.

**Responses:**

| Status | Scenario | Body |
|--------|----------|------|
| 200 | Voted | `{ "flow_id":"...", "user_id":"...", "vote_type":"up", "created_at":"...", "updated_at":"..." }` |
| 400 | Missing/invalid vote_type | `{ "error": "..." }` |

**Backend query:**
```sql
INSERT INTO public.flow_votes (flow_id, user_id, vote_type, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (flow_id, user_id)
DO UPDATE SET vote_type=$3, updated_at=$5
```

---

## WebSocket

| Method | Path | Auth |
|--------|------|------|
| GET | `/ws` | JWT query param |

### GET `/ws`

**Purpose:** Upgrade to WebSocket for real-time board collaboration. Requires Redis.

**Query params:** `?token=<jwt>`

**Behaviour:** Hub publishes events to all clients subscribed to the same board. Disabled when `REDIS_URL` is not set.

---

## Health

### GET `/health`

**Purpose:** Liveness check.

**Response:** `{ "status": "ok" }`

---

## Common Error Codes

| HTTP | AppError Code | When |
|------|---------------|------|
| 400 | BAD_REQUEST | Validation failure / missing required field |
| 401 | UNAUTHORIZED | Missing or expired JWT |
| 403 | FORBIDDEN | Caller lacks org membership or role |
| 404 | NOT_FOUND | Resource does not exist |
| 409 | CONFLICT | Duplicate resource / version mismatch |
| 500 | INTERNAL | Unhandled DB or server error |

---

## Middleware Stack

| Group | Middleware applied |
|-------|--------------------|
| Public auth routes | none |
| `/api/organization/invite/verify` | none |
| `jwtOnly` group | `JWTAuth` → `OrgContext` |
| `protected` group | `JWTAuth` → `OrgContext` → `Encryption` (AES-256-GCM) |

**JWTAuth** validates `Authorization: Bearer <token>`, sets `user_id` on context.  
**OrgContext** looks up the user's active org membership, sets `org_id` + `org_role` on context. Does **not** abort if the user has no org (onboarding is allowed).  
**Encryption** decrypts incoming `{"data":"<base64>"}` body before the handler runs, and re-encrypts the response. Uses a per-session key issued at login (`session_key` in auth response).
