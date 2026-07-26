/**
 * SyncSpace API Integration Test Suite
 * ------------------------------------
 * Tests every backend endpoint in user-flow order.
 *
 * Requirements: Node >= 18 (built-in fetch + webcrypto)
 *
 * Usage:
 *   node api-test.mjs
 *
 * Override base URL:
 *   SYNCSPACE_API=http://localhost:9090 node api-test.mjs
 */

import { webcrypto } from "node:crypto";

// ── Config ────────────────────────────────────────────────────────────────────
const API          = process.env.SYNCSPACE_API ?? "http://localhost:8080";
const TEST_EMAIL   = `testuser_${Date.now()}@syncspace.test`;
const TEST_PASS    = "TestPass123!";
const TEST_NAME    = "Test User";

// ── State (populated as tests run) ───────────────────────────────────────────
let JWT         = "";
let SESSION_KEY = "";
let USER_ID     = "";
let ORG_ID      = "";
let BOARD_ID    = "";
let TASK_ID     = "";
let FLOW_ID     = "";
let MEMBER_ID   = "";
let NOTIF_ID    = "";
let FLOW_VERSION = 0;

// ── Colour helpers ────────────────────────────────────────────────────────────
const GREEN  = (s) => `\x1b[32m${s}\x1b[0m`;
const RED    = (s) => `\x1b[31m${s}\x1b[0m`;
const YELLOW = (s) => `\x1b[33m${s}\x1b[0m`;
const BOLD   = (s) => `\x1b[1m${s}\x1b[0m`;
const DIM    = (s) => `\x1b[2m${s}\x1b[0m`;

// ── Results tracking ──────────────────────────────────────────────────────────
const results = [];
let passed = 0, failed = 0, skipped = 0;

function logResult(label, status, detail = "") {
  const icon   = status === "PASS" ? GREEN("✓") : status === "SKIP" ? YELLOW("○") : RED("✗");
  const colour = status === "PASS" ? GREEN    : status === "SKIP" ? YELLOW    : RED;
  console.log(`  ${icon}  ${colour(status)}  ${label}`);
  if (detail) console.log(`         ${DIM(detail)}`);
  results.push({ label, status, detail });
  if (status === "PASS")      passed++;
  else if (status === "FAIL") failed++;
  else                        skipped++;
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────
function authHeaders(extra = {}) {
  return {
    "Content-Type": "application/json",
    ...(JWT ? { Authorization: `Bearer ${JWT}` } : {}),
    ...extra,
  };
}

async function post(path, body, headers = {}) {
  return fetch(`${API}${path}`, {
    method:  "POST",
    headers: { ...authHeaders(), ...headers },
    body:    JSON.stringify(body),
  });
}

async function get(path, headers = {}) {
  return fetch(`${API}${path}`, {
    method:  "GET",
    headers: { ...authHeaders(), ...headers },
  });
}

async function patch(path, body, headers = {}) {
  return fetch(`${API}${path}`, {
    method:  "PATCH",
    headers: { ...authHeaders(), ...headers },
    body:    JSON.stringify(body),
  });
}

async function put(path, body, headers = {}) {
  return fetch(`${API}${path}`, {
    method:  "PUT",
    headers: { ...authHeaders(), ...headers },
    body:    JSON.stringify(body),
  });
}

async function del(path, headers = {}) {
  return fetch(`${API}${path}`, {
    method:  "DELETE",
    headers: { ...authHeaders(), ...headers },
  });
}

async function parseJson(res) {
  try { return await res.json(); } catch { return null; }
}

// ── AES-256-GCM helpers ───────────────────────────────────────────────────────
// Wire format: base64(12-byte-nonce || ciphertext || 16-byte-GCM-tag)
//
// NOTE: Do NOT destructure webcrypto.subtle or webcrypto.getRandomValues —
// the methods require `this` to be the Crypto object (Node.js WebCrypto).

function b64ToBytes(b64) {
  return Buffer.from(b64, "base64");
}
function bytesToB64(buf) {
  return Buffer.from(buf).toString("base64");
}

async function importAesKey(b64Key) {
  return webcrypto.subtle.importKey(
    "raw",
    b64ToBytes(b64Key),
    { name: "AES-GCM" },
    false,
    ["encrypt", "decrypt"],
  );
}

async function encryptPayload(b64Key, body) {
  const key    = await importAesKey(b64Key);
  const nonce  = webcrypto.getRandomValues(new Uint8Array(12));
  const plain  = new TextEncoder().encode(JSON.stringify(body));
  const cipher = await webcrypto.subtle.encrypt({ name: "AES-GCM", iv: nonce }, key, plain);
  const out    = new Uint8Array(12 + cipher.byteLength);
  out.set(nonce, 0);
  out.set(new Uint8Array(cipher), 12);
  return bytesToB64(out.buffer);
}

async function decryptPayload(b64Key, b64Data) {
  const key    = await importAesKey(b64Key);
  const raw    = b64ToBytes(b64Data);
  const nonce  = raw.slice(0, 12);
  const cipher = raw.slice(12);
  const plain  = await webcrypto.subtle.decrypt({ name: "AES-GCM", iv: nonce }, key, cipher);
  return JSON.parse(new TextDecoder().decode(plain));
}

// Encrypted board requests: response envelope is {"data":"<base64>"}
// Decrypting that gives the original {"success":true,"data":{...}} envelope.
async function decryptResponse(res) {
  const outer = await parseJson(res);
  if (!outer?.data) return null;
  try {
    const inner = await decryptPayload(SESSION_KEY, outer.data);
    if (!inner?.success) return null;
    return inner.data;
  } catch {
    return null;
  }
}

async function encryptedGet(path) {
  // GET has no request body — the middleware skips decryption but still
  // encrypts the response.
  return fetch(`${API}${path}`, {
    method:  "GET",
    headers: {
      "Content-Type":  "application/json",
      Authorization:   `Bearer ${JWT}`,
      "X-User-ID":     USER_ID,
    },
  });
}

async function encryptedPost(path, body) {
  const encrypted = await encryptPayload(SESSION_KEY, body);
  return fetch(`${API}${path}`, {
    method:  "POST",
    headers: {
      "Content-Type":  "application/json",
      Authorization:   `Bearer ${JWT}`,
      "X-User-ID":     USER_ID,
    },
    body: JSON.stringify({ data: encrypted }),
  });
}

// ── Test runner ───────────────────────────────────────────────────────────────
async function test(label, fn) {
  try {
    await fn();
  } catch (err) {
    logResult(label, "FAIL", err.message);
  }
}

// ── TEST SUITE ────────────────────────────────────────────────────────────────
async function run() {
  console.log(BOLD(`\nSyncSpace API Test Suite`));
  console.log(DIM(`Base URL:   ${API}`));
  console.log(DIM(`Test email: ${TEST_EMAIL}\n`));

  // ── 0. Connectivity ───────────────────────────────────────────────────────
  console.log(BOLD("0. Connectivity"));
  await test("Backend is reachable", async () => {
    const res = await fetch(`${API}/health`);
    if (!res) throw new Error("No response");
    logResult("Backend is reachable", "PASS", `HTTP ${res.status}`);
  });

  // ── 1. Auth ───────────────────────────────────────────────────────────────
  console.log(BOLD("\n1. Authentication"));

  await test("POST /api/auth/register", async () => {
    const res  = await post("/api/auth/register", { name: TEST_NAME, email: TEST_EMAIL, password: TEST_PASS });
    const json = await parseJson(res);
    if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
    JWT         = json.data.token;
    SESSION_KEY = json.data.session_key;
    USER_ID     = json.data.user?.id;
    logResult("POST /api/auth/register", "PASS", `user.id=${USER_ID}`);
  });

  await test("POST /api/auth/signin", async () => {
    const res  = await post("/api/auth/signin", { email: TEST_EMAIL, password: TEST_PASS });
    const json = await parseJson(res);
    if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
    JWT         = json.data.token;
    SESSION_KEY = json.data.session_key;
    USER_ID     = json.data.user?.id;
    logResult("POST /api/auth/signin", "PASS", `token.length=${JWT.length}`);
  });

  // ── 2. User Profile ───────────────────────────────────────────────────────
  console.log(BOLD("\n2. User Profile"));

  await test("GET /api/user/me", async () => {
    const res  = await get("/api/user/me");
    const json = await parseJson(res);
    if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
    logResult("GET /api/user/me", "PASS", `email=${json.data.email}`);
  });

  await test("PATCH /api/user/profile", async () => {
    const res  = await patch("/api/user/profile", { name: "Updated Test User" });
    const json = await parseJson(res);
    if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
    logResult("PATCH /api/user/profile", "PASS", `name=${json.data.name}`);
  });

  await test("PATCH /api/user/avatar", async () => {
    const res  = await patch("/api/user/avatar", { avatar_url: "https://example.com/avatar.png" });
    const json = await parseJson(res);
    if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
    logResult("PATCH /api/user/avatar", "PASS");
  });

  // ── 3. Organization ───────────────────────────────────────────────────────
  console.log(BOLD("\n3. Organization"));

  // GET /api/organization returns an ARRAY of orgs (all orgs the user belongs to).
  await test("GET /api/organization", async () => {
    const res  = await get("/api/organization");
    const json = await parseJson(res);
    if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
    const orgs = Array.isArray(json.data) ? json.data : [];
    ORG_ID = orgs[0]?.id ?? "";
    logResult("GET /api/organization", "PASS", `count=${orgs.length}  org.id=${ORG_ID}`);
  });

  if (ORG_ID) {
    await test("PATCH /api/organization/:id", async () => {
      const res  = await patch(`/api/organization/${ORG_ID}`, { name: "Test Corp" });
      const json = await parseJson(res);
      if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
      logResult("PATCH /api/organization/:id", "PASS", `name=${json.data.name}`);
    });

    // Members — all endpoints require ?org_id= query param.
    await test("GET /api/organization/members", async () => {
      const res  = await get(`/api/organization/members?org_id=${ORG_ID}`);
      const json = await parseJson(res);
      if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
      const members = Array.isArray(json.data) ? json.data : [];
      MEMBER_ID = members.find(m => m.user_id !== USER_ID)?.user_id ?? "";
      logResult("GET /api/organization/members", "PASS", `count=${members.length}`);
    });

    await test("POST /api/organization/members/invite", async () => {
      const res  = await post("/api/organization/members/invite", {
        org_id: ORG_ID,
        email:  `invite_${Date.now()}@syncspace.test`,
        role:   "member",
      });
      const json = await parseJson(res);
      if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
      MEMBER_ID = json.data.user_id ?? json.data.id ?? MEMBER_ID;
      logResult("POST /api/organization/members/invite", "PASS", `member.id=${MEMBER_ID}`);
    });

    if (MEMBER_ID) {
      await test("PATCH /api/organization/members/:id/role", async () => {
        const res  = await patch(
          `/api/organization/members/${MEMBER_ID}/role?org_id=${ORG_ID}`,
          { role: "viewer" },
        );
        const json = await parseJson(res);
        if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
        logResult("PATCH /api/organization/members/:id/role", "PASS", `role=${json.data.role}`);
      });

      await test("DELETE /api/organization/members/:id", async () => {
        const res  = await del(`/api/organization/members/${MEMBER_ID}?org_id=${ORG_ID}`);
        const json = await parseJson(res);
        if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
        logResult("DELETE /api/organization/members/:id", "PASS");
      });
    } else {
      logResult("PATCH /api/organization/members/:id/role", "SKIP", "No secondary member found");
      logResult("DELETE /api/organization/members/:id",     "SKIP", "No secondary member found");
    }
  } else {
    logResult("PATCH /api/organization/:id",              "SKIP", "ORG_ID not set — user has no org");
    logResult("GET /api/organization/members",            "SKIP", "ORG_ID not set");
    logResult("POST /api/organization/members/invite",    "SKIP", "ORG_ID not set");
    logResult("PATCH /api/organization/members/:id/role", "SKIP", "ORG_ID not set");
    logResult("DELETE /api/organization/members/:id",     "SKIP", "ORG_ID not set");
  }

  // ── 4. Boards (AES-256-GCM encrypted) ────────────────────────────────────
  console.log(BOLD("\n4. Boards  [AES-256-GCM encrypted]"));

  if (!SESSION_KEY || !USER_ID) {
    logResult("POST /api/boards (encrypted)", "SKIP", "session key not available");
    logResult("GET  /api/boards (encrypted)", "SKIP", "session key not available");
  } else {
    await test("POST /api/boards (encrypted)", async () => {
      const res  = await encryptedPost("/api/boards", {
        title:       "Test Board",
        description: "Created by API test",
      });
      const data = await decryptResponse(res);
      if (!data) {
        const raw = await res.clone().json().catch(() => null);
        throw new Error(raw?.error ?? `HTTP ${res.status} — decryption failed`);
      }
      BOARD_ID = data.id ?? "";
      logResult("POST /api/boards (encrypted)", "PASS", `board.id=${BOARD_ID}`);
    });

    await test("GET /api/boards (encrypted)", async () => {
      const res  = await encryptedGet("/api/boards");
      const data = await decryptResponse(res);
      if (!data) {
        throw new Error(`HTTP ${res.status} — decryption failed`);
      }
      const boards = Array.isArray(data) ? data : [data];
      if (!BOARD_ID && boards.length > 0) BOARD_ID = boards[0].id;
      logResult("GET /api/boards (encrypted)", "PASS", `count=${boards.length}`);
    });
  }

  // ── 5. Board Tasks ────────────────────────────────────────────────────────
  // Stage values:    todo | in_progress | done
  // Priority values: low | medium | high
  // Date format:     RFC3339 e.g. "2025-06-15T00:00:00Z"
  // Assignee field:  assignee_id (user UUID, not a display name)
  console.log(BOLD("\n5. Board Tasks"));

  if (!BOARD_ID) {
    for (const lbl of ["GET","POST","PATCH title","PATCH priority","PATCH due_date","PATCH assignee_id","PATCH stage","DELETE"])
      logResult(`${lbl} /api/tasks`, "SKIP", "BOARD_ID not set");
  } else {
    await test("GET /api/boards/:id/tasks", async () => {
      const res  = await get(`/api/boards/${BOARD_ID}/tasks`);
      const json = await parseJson(res);
      if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
      const tasks = Array.isArray(json.data) ? json.data : [];
      logResult("GET /api/boards/:id/tasks", "PASS", `count=${tasks.length}`);
    });

    await test("POST /api/boards/:id/tasks", async () => {
      const res  = await post(`/api/boards/${BOARD_ID}/tasks`, {
        title:       "API Test Task",
        description: "Created by api-test.mjs",
        stage:       "todo",         // must be: todo | in_progress | done
        priority:    "high",         // must be: low | medium | high
        due_date:    "2025-06-15T00:00:00Z", // RFC3339; omit for no due date
        // assignee_id omitted — must be a valid user UUID if provided
      });
      const json = await parseJson(res);
      if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
      TASK_ID = json.data.id;
      logResult("POST /api/boards/:id/tasks", "PASS", `task.id=${TASK_ID}`);
    });

    if (TASK_ID) {
      await test("PATCH /api/tasks/:id  (title)", async () => {
        const res  = await patch(`/api/tasks/${TASK_ID}`, { title: "Updated Task Title" });
        const json = await parseJson(res);
        if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
        logResult("PATCH /api/tasks/:id  (title)", "PASS");
      });

      await test("PATCH /api/tasks/:id  (priority)", async () => {
        const res  = await patch(`/api/tasks/${TASK_ID}`, { priority: "low" });
        const json = await parseJson(res);
        if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
        logResult("PATCH /api/tasks/:id  (priority)", "PASS");
      });

      await test("PATCH /api/tasks/:id  (due_date)", async () => {
        // due_date must be RFC3339; send clear_due_date:true to unset it.
        const res  = await patch(`/api/tasks/${TASK_ID}`, { due_date: "2025-06-20T00:00:00Z" });
        const json = await parseJson(res);
        if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
        logResult("PATCH /api/tasks/:id  (due_date)", "PASS");
      });

      // assignee_id must be a real user UUID; skip if no second user exists.
      logResult("PATCH /api/tasks/:id  (assignee_id)", "SKIP", "requires a valid assignee UUID — omitted in test");

      await test("PATCH /api/tasks/:id/stage", async () => {
        // Requires ?board_id= query param + { stage, position } body.
        const res  = await patch(
          `/api/tasks/${TASK_ID}/stage?board_id=${BOARD_ID}`,
          { stage: "in_progress", position: 0 },
        );
        const json = await parseJson(res);
        if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
        logResult("PATCH /api/tasks/:id/stage", "PASS", `stage=${json.data.stage}`);
      });

      await test("DELETE /api/tasks/:id", async () => {
        const res  = await del(`/api/tasks/${TASK_ID}`);
        const json = await parseJson(res);
        if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
        logResult("DELETE /api/tasks/:id", "PASS");
      });
    } else {
      for (const f of ["title","priority","due_date","assignee_id","stage","DELETE"])
        logResult(`PATCH /api/tasks/:id (${f})`, "SKIP", "TASK_ID not set");
    }
  }

  // ── 6. Linked Resources ───────────────────────────────────────────────────
  console.log(BOLD("\n6. Linked Resources"));

  if (!BOARD_ID) {
    logResult("GET /api/boards/:id/linked-resources", "SKIP", "BOARD_ID not set");
    logResult("PUT /api/boards/:id/linked-resources", "SKIP", "BOARD_ID not set");
  } else {
    await test("GET /api/boards/:id/linked-resources", async () => {
      const res  = await get(`/api/boards/${BOARD_ID}/linked-resources`);
      const json = await parseJson(res);
      if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
      logResult("GET /api/boards/:id/linked-resources", "PASS");
    });

    await test("PUT /api/boards/:id/linked-resources", async () => {
      const res  = await put(`/api/boards/${BOARD_ID}/linked-resources`, {
        description:        "Board linked resources",
        documentationLinks: [{ title: "Docs",  url: "https://docs.example.com" }],
        links:              [{ title: "Figma", url: "https://figma.com"        }],
      });
      const json = await parseJson(res);
      if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
      logResult("PUT /api/boards/:id/linked-resources", "PASS");
    });
  }

  // ── 7. Flow Diagrams ──────────────────────────────────────────────────────
  // Boards do not return a flow_id — flows are a separate entity.
  // Obtain FLOW_ID out-of-band (e.g. from the DB or a future board-detail endpoint).
  console.log(BOLD("\n7. Flow Diagrams"));

  if (!FLOW_ID) {
    logResult("GET /api/flows/:id",        "SKIP", "FLOW_ID not known — set FLOW_ID manually to test");
    logResult("PUT /api/flows/:id",        "SKIP", "FLOW_ID not known");
    logResult("GET /api/flows/:id/votes",  "SKIP", "FLOW_ID not known");
    logResult("POST /api/flows/:id/votes", "SKIP", "FLOW_ID not known");
  } else {
    await test("GET /api/flows/:id", async () => {
      const res  = await get(`/api/flows/${FLOW_ID}`);
      const json = await parseJson(res);
      if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
      FLOW_VERSION = json.data.version ?? 0;
      logResult("GET /api/flows/:id", "PASS",
        `nodes=${json.data.data?.nodes?.length ?? 0}  version=${FLOW_VERSION}`);
    });

    await test("PUT /api/flows/:id", async () => {
      // Body: { data: {nodes, edges}, version: <current> }
      // version must match the stored version (optimistic lock).
      const res  = await put(`/api/flows/${FLOW_ID}`, {
        data:    { nodes: [{ id: "1", type: "shape", position: { x: 100, y: 100 },
                             data: { type: "rectangle", text: "Start", fill: "#3B82F6" } }],
                   edges: [] },
        version: FLOW_VERSION,
      });
      const json = await parseJson(res);
      if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
      FLOW_VERSION = json.data.version;
      logResult("PUT /api/flows/:id", "PASS", `new version=${FLOW_VERSION}`);
    });

    await test("GET /api/flows/:id/votes", async () => {
      const res  = await get(`/api/flows/${FLOW_ID}/votes`);
      const json = await parseJson(res);
      if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
      logResult("GET /api/flows/:id/votes", "PASS",
        `up=${json.data.up}  down=${json.data.down}  my_vote="${json.data.my_vote}"`);
    });

    await test("POST /api/flows/:id/votes", async () => {
      // vote_type must be "up" or "down".
      const res  = await post(`/api/flows/${FLOW_ID}/votes`, { vote_type: "up" });
      const json = await parseJson(res);
      if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
      logResult("POST /api/flows/:id/votes", "PASS", `vote_type=${json.data.vote_type}`);
    });
  }

  // ── 8. Dashboard & Analytics ──────────────────────────────────────────────
  console.log(BOLD("\n8. Dashboard & Analytics"));

  const dashEndpoints = [
    { label: "GET /api/dashboard/stats",                 path: "/api/dashboard/stats"               },
    { label: "GET /api/boards/recent",                   path: "/api/boards/recent"                  },
    { label: "GET /api/tasks/upcoming",                  path: "/api/tasks/upcoming"                 },
    { label: "GET /api/activity-logs",                   path: "/api/activity-logs"                  },
    { label: "GET /api/analytics/task-completion",       path: "/api/analytics/task-completion"      },
    { label: "GET /api/analytics/task-distribution",     path: "/api/analytics/task-distribution"    },
    { label: "GET /api/analytics/board-activity",        path: "/api/analytics/board-activity"       },
    { label: "GET /api/analytics/team-contribution",     path: "/api/analytics/team-contribution"    },
  ];

  for (const ep of dashEndpoints) {
    await test(ep.label, async () => {
      const res  = await get(ep.path);
      const json = await parseJson(res);
      if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
      logResult(ep.label, "PASS");
    });
  }

  // ── 9. Notifications ──────────────────────────────────────────────────────
  console.log(BOLD("\n9. Notifications"));

  await test("GET /api/notifications", async () => {
    const res  = await get("/api/notifications");
    const json = await parseJson(res);
    if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
    const notifs = json.data?.notifications ?? (Array.isArray(json.data) ? json.data : []);
    NOTIF_ID = notifs[0]?.id ?? "";
    logResult("GET /api/notifications", "PASS",
      `count=${notifs.length}  unread=${json.data?.unread_count ?? 0}`);
  });

  if (NOTIF_ID) {
    await test("PATCH /api/notifications/:id/read", async () => {
      const res  = await patch(`/api/notifications/${NOTIF_ID}/read`);
      const json = await parseJson(res);
      if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
      logResult("PATCH /api/notifications/:id/read", "PASS");
    });
  } else {
    logResult("PATCH /api/notifications/:id/read", "SKIP", "No notifications returned");
  }

  await test("PATCH /api/notifications/mark-all-read", async () => {
    const res  = await patch("/api/notifications/mark-all-read");
    const json = await parseJson(res);
    if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
    logResult("PATCH /api/notifications/mark-all-read", "PASS",
      `marked_read=${json.data?.marked_read ?? 0}`);
  });

  // ── 10. Logout ────────────────────────────────────────────────────────────
  console.log(BOLD("\n10. Auth — Logout"));

  await test("POST /api/auth/logout", async () => {
    const res  = await post("/api/auth/logout", {});
    const json = await parseJson(res);
    if (!json?.success) throw new Error(json?.error ?? `HTTP ${res.status}`);
    logResult("POST /api/auth/logout", "PASS");
  });

  // ── Summary ───────────────────────────────────────────────────────────────
  const total = passed + failed + skipped;
  console.log(BOLD("\n══════════════════════════════════════════"));
  console.log(BOLD("Results"));
  console.log(
    `  ${GREEN(`${passed} passed`)}   ${failed > 0 ? RED(`${failed} failed`) : `${failed} failed`}` +
    `   ${YELLOW(`${skipped} skipped`)}   ${total} total`,
  );

  if (failed > 0) {
    console.log(BOLD("\nFailed tests:"));
    results
      .filter(r => r.status === "FAIL")
      .forEach(r => console.log(`  ${RED("✗")}  ${r.label}${r.detail ? `\n     ${DIM(r.detail)}` : ""}`));
  }

  console.log(BOLD("══════════════════════════════════════════\n"));
  process.exit(failed > 0 ? 1 : 0);
}

run().catch(err => {
  console.error(RED(`\nFatal: ${err.message}\n`));
  process.exit(1);
});
