-- Codeit local schema (PostgreSQL)
-- Run with: psql "postgres://postgres:postgres@localhost:5432/codeit?sslmode=disable" -f scripts/schema.sql

BEGIN;

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    avatar_url TEXT NULL,
    rating INTEGER NOT NULL DEFAULT 1200 CHECK (rating >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Backward-compatible migration if users table already exists.
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT;
ALTER TABLE users ALTER COLUMN avatar_url SET DEFAULT '';

UPDATE users SET avatar_url = '' WHERE avatar_url IS NULL;

CREATE TABLE IF NOT EXISTS problems (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    difficulty TEXT NOT NULL CHECK (difficulty IN ('easy', 'medium', 'hard')),
    time_limit_ms INTEGER NOT NULL CHECK (time_limit_ms > 0),
    memory_limit_mb INTEGER NOT NULL CHECK (memory_limit_mb > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS test_cases (
    id BIGSERIAL PRIMARY KEY,
    problem_id BIGINT NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    input TEXT NOT NULL,
    expected TEXT NOT NULL,
    is_sample BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS matches (
    id TEXT PRIMARY KEY,
    player1_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    player2_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    problem_id BIGINT NOT NULL REFERENCES problems(id) ON DELETE RESTRICT,
    status TEXT NOT NULL CHECK (status IN ('waiting', 'running', 'finished')),
    duration_seconds INTEGER NOT NULL CHECK (duration_seconds > 0),
    started_at TIMESTAMPTZ NULL,
    ended_at TIMESTAMPTZ NULL,
    winner_id TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (player1_id <> player2_id)
);

ALTER TABLE matches ADD COLUMN IF NOT EXISTS victory_type TEXT NULL;

-- Elo snapshot per match (filled when ratings.ApplyFinishedMatch runs; NULL for older rows).
ALTER TABLE matches ADD COLUMN IF NOT EXISTS player1_rating_after INTEGER NULL;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS player2_rating_after INTEGER NULL;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS player1_elo_delta INTEGER NULL;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS player2_elo_delta INTEGER NULL;

-- Friend / casual duels: when true, ratings.ApplyFinishedMatch is a no-op for this match.
ALTER TABLE matches ADD COLUMN IF NOT EXISTS skip_elo BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS match_invites (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    host_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('pending', 'accepted')),
    difficulty TEXT NOT NULL CHECK (difficulty IN ('easy', 'medium', 'hard')),
    duration_seconds INTEGER NOT NULL CHECK (duration_seconds > 0),
    skip_elo BOOLEAN NOT NULL DEFAULT TRUE,
    match_id TEXT NULL REFERENCES matches(id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_match_invites_code ON match_invites(code);
CREATE INDEX IF NOT EXISTS idx_match_invites_host_status ON match_invites(host_user_id, status);

CREATE TABLE IF NOT EXISTS submissions (
    id TEXT PRIMARY KEY,
    match_id TEXT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    language TEXT NOT NULL,
    code TEXT NOT NULL,
    passed_count INTEGER NOT NULL CHECK (passed_count >= 0),
    total_count INTEGER NOT NULL CHECK (total_count >= 0),
    status TEXT NOT NULL,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS match_analyses (
    id TEXT PRIMARY KEY,
    match_id TEXT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    submission_id TEXT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    language TEXT NOT NULL,
    passed_count INTEGER NOT NULL CHECK (passed_count >= 0),
    total_count INTEGER NOT NULL CHECK (total_count >= 0),
    summary TEXT NOT NULL,
    strengths JSONB NOT NULL DEFAULT '[]'::jsonb,
    issues JSONB NOT NULL DEFAULT '[]'::jsonb,
    suggestions JSONB NOT NULL DEFAULT '[]'::jsonb,
    score DOUBLE PRECISION NULL,
    analyzed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE match_analyses ALTER COLUMN submission_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_problems_difficulty ON problems(difficulty);
CREATE INDEX IF NOT EXISTS idx_test_cases_problem_id ON test_cases(problem_id);
CREATE INDEX IF NOT EXISTS idx_matches_user_status_created ON matches(player1_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_matches_user2_status_created ON matches(player2_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_submissions_match_user ON submissions(match_id, user_id);
CREATE INDEX IF NOT EXISTS idx_match_analyses_user_time ON match_analyses(user_id, analyzed_at DESC);
CREATE INDEX IF NOT EXISTS idx_match_analyses_match_user_time ON match_analyses(match_id, user_id, analyzed_at DESC);

COMMIT;

-- Optional seed data for quick local testing.
INSERT INTO problems (title, description, difficulty, time_limit_ms, memory_limit_mb)
SELECT
    'Sum Two Numbers',
    'Read two integers and print their sum.',
    'easy',
    1000,
    128
WHERE NOT EXISTS (
    SELECT 1 FROM problems WHERE title = 'Sum Two Numbers'
);

INSERT INTO test_cases (problem_id, input, expected, is_sample)
SELECT p.id, tc.input, tc.expected, tc.is_sample
FROM (
    VALUES
        ('1 2', '3', TRUE),
        ('10 20', '30', TRUE),
        ('100 200', '300', FALSE)
) AS tc(input, expected, is_sample)
JOIN problems p ON p.title = 'Sum Two Numbers'
WHERE NOT EXISTS (
    SELECT 1
    FROM test_cases t
    WHERE t.problem_id = p.id
);
