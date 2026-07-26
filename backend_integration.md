# SyncSpace Backend Integration Guide

> Instructions for Claude Code: Replace all `setTimeout`-based mock operations in Zustand stores
> with real `fetch` calls to the Go backend. Follow this guide exactly — it reflects the actual
> backend implementation, which differs from the earlier API spec in SYNCSPACE_SUMMARY.md.

---

## Local Development

Backend runs at: `http://localhost:8080`

**There is no `/v1` prefix.** All API paths start with `/api/`.

Add this to `.env.local` in the frontend:
```
NEXT_PUBLIC_API_BASE=http://localhost:8080
```

Read it in code:
```ts
const API = process.env.NEXT_PUBLIC_API_BASE ?? 'http://localhost:8080';
```

---

## Auth Architecture — Read This First

Login returns **two tokens**. Both must be stored and used:

| Token | Stored in | Used for |
|---|---|---|
| `token` | `localStorage` as `ss_jwt` | `Authorization: Bearer <token>` header on all protected routes |
| `session_key` | `localStorage` as `ss_session_key` | AES-256-GCM encryption on board routes only |
| `user.id` | `localStorage` as `ss_user_id` | `X-User-ID` header on encrypted board routes |

On logout, clear all three keys and call `POST /api/auth/logout`.

---

## Response Envelope

Every endpoint returns:
```json
{ "success": true, "data": { ... } }
{ "success": false, "error": "msg" }
```

Always read from `response.data`, never the response root.

---

## API Utility — Create `src/lib/api.ts`

```ts
const API = process.env.NEXT_PUBLIC_API_BASE ?? 'http://localhost:8080';

function getHeaders(extra: Record<string, string> = {}): HeadersInit {
  const jwt = localStorage.getItem('ss_jwt');
  const headers: Record<string, string> = { 'Content-Type': 'application/json', ...extra };
  if (jwt) headers['Authorization'] = `Bearer ${jwt}`;
  return headers;
}

export async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const res = await fetch(`${API}${path}`, {
    ...options,
    headers: getHeaders(options.headers as Record<string, string>),
  });
  const json = await res.json();
  if (!json.success) throw new Error(json.error ?? 'Unknown error');
  return json.data as T;
}
```

---

## Encrypted Routes — Board List & Create Only

**Only two routes use AES-256-GCM payload encryption:**
- `GET /api/boards`
- `POST /api/boards`

All other routes send and receive **plaintext JSON**.

### Encryption utility — create `src/lib/crypto.ts`

Uses the browser's built-in Web Crypto API. The backend wire format is:
`base64( 12-byte-nonce || ciphertext || 16-byte-GCM-tag )`

```ts
function b64ToBytes(b64: string): Uint8Array {
  return Uint8Array.from(atob(b64), c => c.charCodeAt(0));
}
function bytesToB64(buf: ArrayBuffer): string {
  return btoa(String.fromCharCode(...new Uint8Array(buf)));
}

async function importKey(b64Key: string): Promise<CryptoKey> {
  return crypto.subtle.importKey(
    'raw', b64ToBytes(b64Key), { name: 'AES-GCM' }, false, ['encrypt', 'decrypt'],
  );
}

export async function encryptPayload(b64Key: string, body: object): Promise<string> {
  const key = await importKey(b64Key);
  const nonce = crypto.getRandomValues(new Uint8Array(12));
  const plain = new TextEncoder().encode(JSON.stringify(body));
  const cipher = await crypto.subtle.encrypt({ name: 'AES-GCM', iv: nonce }, key, plain);
  const out = new Uint8Array(12 + cipher.byteLength);
  out.set(nonce, 0);
  out.set(new Uint8Array(cipher), 12);
  return bytesToB64(out.buffer);
}

export async function decryptPayload(b64Key: string, b64Data: string): Promise<unknown> {
  const key = await importKey(b64Key);
  const raw = b64ToBytes(b64Data);
  const nonce = raw.slice(0, 12);
  const cipher = raw.slice(12);
  const plain = await crypto.subtle.decrypt({ name: 'AES-GCM', iv: nonce }, key, cipher);
  return JSON.parse(new TextDecoder().decode(plain));
}
```

### Encrypted fetch helper — add to `src/lib/api.ts`

```ts
import { encryptPayload, decryptPayload } from './crypto';

export async function encryptedFetch<T>(
  path: string,
  method: 'GET' | 'POST',
  body?: object,
): Promise<T> {
  const jwt = localStorage.getItem('ss_jwt');
  const sessionKey = localStorage.getItem('ss_session_key');
  const userId = localStorage.getItem('ss_user_id');

  if (!jwt || !sessionKey || !userId) throw new Error('Not authenticated');

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${jwt}`,
    'X-User-ID': userId,
  };

  let fetchBody: string | undefined;
  if (body) {
    const encrypted = await encryptPayload(sessionKey, body);
    fetchBody = JSON.stringify({ data: encrypted });
  }

  const res = await fetch(`${API}${path}`, { method, headers, body: fetchBody });
  const json = await res.json();
  if (!json.success) throw new Error(json.error ?? 'Unknown error');

  const decrypted = await decryptPayload(sessionKey, (json.data as { data: string }).data);
  return decrypted as T;
}
```

---

## Endpoint Reference — Actual Backend Paths

> ⚠️ These paths differ from SYNCSPACE_SUMMARY.md. Use these exclusively.

### Auth (public — no JWT required)

| Action | Method | Path | Notes |
|---|---|---|---|
| Register | POST | `/api/auth/register` | Body: `{ name, email, password }` |
| Sign in | POST | `/api/auth/signin` | Body: `{ email, password }` |
| Logout | POST | `/api/auth/logout` | Requires JWT; revokes session key |

**Register / signin response shape** (inside `data`):
```json
{
  "token": "eyJ...",
  "session_key": "base64AES256Key==",
  "user": {
    "id": "...", "name": "...", "email": "...",
    "avatar_url": "...", "created_at": "...", "updated_at": "..."
  }
}
```

On success: store `token` → `ss_jwt`, `session_key` → `ss_session_key`, `user.id` → `ss_user_id`.

---

### User Profile (JWT only, plaintext)

| Action | Method | Path | Body |
|---|---|---|---|
| Get current user | GET | `/api/user/me` | — |
| Update name / email | PATCH | `/api/user/profile` | `{ name?, email? }` — omit fields that are not changing |
| Update avatar URL | PATCH | `/api/user/avatar` | `{ avatar_url: "https://..." }` |

**User field name mapping (backend snake_case → frontend camelCase):**

| Backend | Frontend |
|---|---|
| `avatar_url` | `avatar` |
| `created_at` | `createdAt` |
| `updated_at` | `updatedAt` |

---

### Organization (JWT only, plaintext)

| Action | Method | Path | Body |
|---|---|---|---|
| Get org | GET | `/api/organization` | — |
| Update org name | PATCH | `/api/organization/:id` | `{ name }` |
| List members | GET | `/api/organization/members` | — |
| Invite member | POST | `/api/organization/members/invite` | `{ email, role }` |
| Update member role | PATCH | `/api/organization/members/:id/role` | `{ role }` |
| Remove member | DELETE | `/api/organization/members/:id` | — |

---

### Boards (JWT + AES encryption — use `encryptedFetch`)

| Action | Method | Path |
|---|---|---|
| List boards | GET | `/api/boards` |
| Create board | POST | `/api/boards` |

These are the **only two** encrypted endpoints. Use `encryptedFetch`, not `apiFetch`.

**Create board body:** `{ title, description, coverColor }`

---

### Board Tasks (JWT only, plaintext)

| Action | Method | Path | Body |
|---|---|---|---|
| List tasks for board | GET | `/api/boards/:id/tasks` | — |
| Create task | POST | `/api/boards/:id/tasks` | `{ stage, title, tags, startDate, endDate, assignee, priority, flagged }` |
| Update task fields | PATCH | `/api/tasks/:id` | Any subset of task fields (send only changed fields) |
| Move task to stage | PATCH | `/api/tasks/:id/stage` | `{ stage }` |
| Delete task | DELETE | `/api/tasks/:id` | — |

> The frontend spec had per-field endpoints (`/tasks/:id/title`, `/tasks/:id/priority`, etc.).
> The backend consolidates all field updates into a single `PATCH /api/tasks/:id`.
> For stage changes use the dedicated `/stage` endpoint (drag-and-drop optimised).

---

### Linked Resources (JWT only, plaintext)

| Action | Method | Path |
|---|---|---|
| Get board resources | GET | `/api/boards/:id/linked-resources` |
| Save all resources | PUT | `/api/boards/:id/linked-resources` |

`:id` is the board ID. PUT body: `{ description, documentationLinks, links }`.

---

### Flow Diagrams (JWT only, plaintext)

| Action | Method | Path | Body |
|---|---|---|---|
| Get diagram | GET | `/api/flows/:id` | — |
| Save diagram | PUT | `/api/flows/:id` | `{ title, nodes, edges }` |
| Get votes | GET | `/api/flows/:id/votes` | — |
| Cast / update vote | POST | `/api/flows/:id/votes` | `{ vote: "approve" \| "review" \| "reject" }` |

> There is no `DELETE /votes/me` endpoint yet. Re-casting the same vote type acts as a toggle
> (backend handles idempotency); casting a different type replaces the existing vote.
>
> There is no `POST /api/flows` (create blank diagram) endpoint yet. Always save to an existing
> flow ID retrieved from the board context.

---

### Dashboard (JWT only, plaintext)

| Action | Method | Path |
|---|---|---|
| Aggregated stats | GET | `/api/dashboard/stats` |
| Recent boards | GET | `/api/boards/recent` |
| Upcoming tasks (within 7 days) | GET | `/api/tasks/upcoming` |
| Activity feed | GET | `/api/activity-logs` |

---

### Notifications (JWT only, plaintext)

| Action | Method | Path |
|---|---|---|
| List notifications | GET | `/api/notifications` |
| Mark one read | PATCH | `/api/notifications/:id/read` |
| Mark all read | PATCH | `/api/notifications/mark-all-read` |

---

### Analytics (JWT only, plaintext)

| Action | Method | Path |
|---|---|---|
| Task completion trend | GET | `/api/analytics/task-completion` |
| Task distribution | GET | `/api/analytics/task-distribution` |
| Board activity | GET | `/api/analytics/board-activity` |
| Team contribution | GET | `/api/analytics/team-contribution` |

---

## Store Migration Map

Replace each `setTimeout` mock with the corresponding real call:

| Store method | Real call |
|---|---|
| load boards | `encryptedFetch<Board[]>('/api/boards', 'GET')` |
| create board | `encryptedFetch<Board>('/api/boards', 'POST', { title, description, coverColor })` |
| load tasks | `apiFetch('/api/boards/:id/tasks')` |
| create task | `apiFetch('/api/boards/:id/tasks', { method: 'POST', body: JSON.stringify({...}) })` |
| update task title | `apiFetch('/api/tasks/:id', { method: 'PATCH', body: JSON.stringify({ title }) })` |
| update task priority | `apiFetch('/api/tasks/:id', { method: 'PATCH', body: JSON.stringify({ priority }) })` |
| update task dates | `apiFetch('/api/tasks/:id', { method: 'PATCH', body: JSON.stringify({ startDate, endDate }) })` |
| update task assignee | `apiFetch('/api/tasks/:id', { method: 'PATCH', body: JSON.stringify({ assignee }) })` |
| move task stage | `apiFetch('/api/tasks/:id/stage', { method: 'PATCH', body: JSON.stringify({ stage }) })` |
| save linked resources | `apiFetch('/api/boards/:id/linked-resources', { method: 'PUT', body: JSON.stringify({...}) })` |
| diagram auto-save | `apiFetch('/api/flows/:id', { method: 'PUT', body: JSON.stringify({ title, nodes, edges }) })` |
| cast vote | `apiFetch('/api/flows/:id/votes', { method: 'POST', body: JSON.stringify({ vote }) })` |
| get votes | `apiFetch('/api/flows/:id/votes')` |

### localStorage keys

| Key | Action |
|---|---|
| `ss_jwt` | Write on login; read for every API call |
| `ss_session_key` | Write on login; read for board encrypted calls |
| `ss_user_id` | Write on login; send as `X-User-ID` on board calls |
| `syncspace-diagram` | Replace write with `PUT /api/flows/:id`; replace read with `GET /api/flows/:id` |
| `syncspace-diagram-votes` | Replace with `GET/POST /api/flows/:id/votes` |
| `syncspace-recent-templates` | Keep local — no backend needed |
| `theme` | Keep local — user device preference |

---

## WebSocket (Real-time)

```
ws://localhost:8080/ws?token=<JWT>
```

JWT goes in the query string (browsers cannot set WebSocket headers).

```ts
const ws = new WebSocket(`ws://localhost:8080/ws?token=${localStorage.getItem('ss_jwt')}`);
```

Use for: live cursor positions, presence indicators, vote sync.
`CursorOverlay.tsx` is already built — uncomment its import in `DiagramFrame.tsx` and replace
the random-walk mock with real WS position events.

---

## Error Handling Pattern

```ts
try {
  setSaveState(`${id}-${field}`, 'loading');
  const result = await apiFetch<ResultType>(...);
  // apply result to store state
  setSaveState(`${id}-${field}`, 'success');
  setTimeout(() => setSaveState(`${id}-${field}`, 'idle'), 1500);
} catch (err) {
  setSaveState(`${id}-${field}`, 'error');
  const msg = (err as Error).message;
  if (msg.includes('401') || msg.toLowerCase().includes('unauthorized')) {
    localStorage.removeItem('ss_jwt');
    localStorage.removeItem('ss_session_key');
    localStorage.removeItem('ss_user_id');
    router.push('/signin');
  }
}
```

| HTTP status | Handling |
|---|---|
| 400 | Show validation message inline |
| 401 | Clear auth keys → redirect `/signin` |
| 403 | Show permission-denied toast |
| 404 | Show not-found state |
| 500 | Show generic error toast |

---

## CORS

The backend must allow the frontend origin. If you see CORS errors in dev:
- `Access-Control-Allow-Origin: http://localhost:3000`
- `Access-Control-Allow-Headers: Authorization, Content-Type, X-User-ID`
- `Access-Control-Allow-Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS`

---

## Implementation Order (Recommended)

1. Create `src/lib/crypto.ts` and `src/lib/api.ts`
2. Wire auth: register + signin → store `ss_jwt`, `ss_session_key`, `ss_user_id`
3. Wire `GET /api/user/me` to hydrate user context on app load
4. Wire board list/create (encrypted) — this validates the full crypto layer end-to-end
5. Migrate task CRUD (`useBoardTaskStore`)
6. Migrate linked resources (`useLinkedResourcesStore`)
7. Wire diagram save/load (`/api/flows/:id`)
8. Wire votes (`/api/flows/:id/votes`)
9. Wire notifications, dashboard, analytics
10. Connect WebSocket for real-time cursor / presence features
