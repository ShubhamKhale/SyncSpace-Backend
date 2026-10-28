-- Attendance tracking + leave management tables (Tier 1).
-- Safe to run more than once (IF NOT EXISTS).
-- Apply: psql "$DB_URL" -f migrations/003_attendance_leave.sql

-- ── 1. attendance_logs ───────────────────────────────────────────────────────
-- One row per check-in session. check_out_at NULL means still checked in.
CREATE TABLE IF NOT EXISTS public.attendance_logs (
    id             TEXT        PRIMARY KEY,
    user_id        TEXT        NOT NULL REFERENCES public.users(id)          ON DELETE CASCADE,
    org_id         TEXT        NOT NULL REFERENCES public.organizations(id)  ON DELETE CASCADE,
    check_in_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    check_out_at   TIMESTAMPTZ,
    note           TEXT        NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_attendance_user_date ON public.attendance_logs (user_id, check_in_at DESC);
CREATE INDEX IF NOT EXISTS idx_attendance_org_date  ON public.attendance_logs (org_id, check_in_at DESC);

-- ── 2. leave_types ───────────────────────────────────────────────────────────
-- Per-org configurable leave categories. Seeded with Sick/Casual/Earned on first access.
CREATE TABLE IF NOT EXISTS public.leave_types (
    id             TEXT        PRIMARY KEY,
    org_id         TEXT        NOT NULL REFERENCES public.organizations(id)  ON DELETE CASCADE,
    name           TEXT        NOT NULL,
    default_days   INT         NOT NULL DEFAULT 12,
    is_active      BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, name)
);

-- ── 3. leave_balances ────────────────────────────────────────────────────────
-- Per user, per leave type, per year. Auto-initialized from leave_types.default_days.
CREATE TABLE IF NOT EXISTS public.leave_balances (
    user_id        TEXT        NOT NULL REFERENCES public.users(id)          ON DELETE CASCADE,
    org_id         TEXT        NOT NULL REFERENCES public.organizations(id)  ON DELETE CASCADE,
    leave_type_id  TEXT        NOT NULL REFERENCES public.leave_types(id)    ON DELETE CASCADE,
    year           INT         NOT NULL,
    allocated      INT         NOT NULL DEFAULT 0,
    used           INT         NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, leave_type_id, year)
);

-- ── 4. leave_requests ────────────────────────────────────────────────────────
-- Create → pending → approved/denied workflow.
CREATE TABLE IF NOT EXISTS public.leave_requests (
    id             TEXT        PRIMARY KEY,
    user_id        TEXT        NOT NULL REFERENCES public.users(id)          ON DELETE CASCADE,
    org_id         TEXT        NOT NULL REFERENCES public.organizations(id)  ON DELETE CASCADE,
    leave_type_id  TEXT        NOT NULL REFERENCES public.leave_types(id)    ON DELETE CASCADE,
    start_date     DATE        NOT NULL,
    end_date       DATE        NOT NULL,
    reason         TEXT        NOT NULL DEFAULT '',
    status         TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'denied')),
    reviewed_by    TEXT        REFERENCES public.users(id),
    reviewed_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (end_date >= start_date)
);
CREATE INDEX IF NOT EXISTS idx_leave_req_user ON public.leave_requests (user_id, start_date DESC);
CREATE INDEX IF NOT EXISTS idx_leave_req_org  ON public.leave_requests (org_id, status, start_date);
