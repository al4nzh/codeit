# Codeit

Backend for a **1v1 coding duel** app: matchmaking, live matches over WebSocket, Judge0-backed submissions, optional post-match analysis, and Elo-style ratings.

For endpoint-by-endpoint detail, see **`docs/API.md`**.

## What you get (features)

- **Auth**
  - **Google SSO** via `POST /api/v1/auth/google` (ID token from Google Identity Services).
  - Legacy **email + password** register/login still exists in the codebase (optional to expose in UI).
- **Matchmaking** (ranked queue) and **friend battles** (shareable invite codes).
- **Matches** with a server-authoritative timer (`started_at` + `duration_seconds`).
- **Submissions** judged through Judge0; realtime updates over WebSocket.
- **Ratings** applied when a ranked match finishes (friend matches can set `skip_elo`).
- **Match chat** over WebSocket (`chat_message` events).
- **Anti-cheat (MVP)**: tab-switch forfeits enforced server-side over WebSocket (`tab_switched`).
- **Avatars**: upload to local disk under `uploads/avatars/` or set an HTTPS URL (validated on update).
- **Username change**: `PATCH /api/v1/me/username` (authenticated).

## Requirements

- **Go** (see `go.mod` for the toolchain version).
- **PostgreSQL** database reachable via `DATABASE_URL`.

## Quick start (local)

### 1) Create the database schema

Apply `scripts/schema.sql` to your Postgres instance (adjust the connection string):

```powershell
psql "postgres://USER:PASS@HOST:5432/DBNAME?sslmode=disable" -f scripts/schema.sql
```

### 2) Configure environment variables

Create a `.env` file (or export variables in your shell). Common keys:

| Variable | Purpose |
|----------|---------|
| `DATABASE_URL` | Postgres connection string (**required**) |
| `JWT_SECRET` | Secret for signing app JWTs (**required**) |
| `GOOGLE_CLIENT_ID` | Google **Web** OAuth client id used to verify Google ID tokens |
| `JUDGE0_BASE_URL` | Judge0 base URL (**required** for submissions) |
| `JUDGE0_API_KEY` | Optional API key / token for Judge0 |
| `JUDGE0_RAPIDAPI_HOST` | Optional RapidAPI host header |
| `PORT` | HTTP port (defaults to `8080`) |
| `CORS_ALLOWED_ORIGINS` | Comma-separated allowed browser origins (defaults include `http://localhost:5173`) |
| `PUBLIC_APP_URL` | Optional; used to build full friend-battle share URLs |

**Windows tip:** `go run` only sees variables that exist in the **current shell session**. If you keep secrets in `.env`, load them before starting the API (or use your own small launcher script).

### 3) Run the API

```powershell
go run ./cmd/codeit
```

Health check:

- `GET http://localhost:8080/health`

## Project layout (where things live)

- `cmd/codeit` — process entrypoint (`main`)
- `internal/app` — wiring: HTTP routes, DB pool, service construction
- `internal/users` — users, profiles, Google login, avatar upload, username change
- `internal/matches` — match persistence + lifecycle helpers
- `internal/matchmaking` — in-memory ranked queue (single-instance MVP)
- `internal/friendbattles` — DB-backed invites + join flow
- `internal/submissions` — Judge0 integration + match resolution hooks
- `internal/ratings` — post-match Elo updates
- `internal/ws` — WebSocket hub (user channels + match rooms)
- `internal/analysis` — optional LLM / analyzer integration + cached analyses
- `docs/API.md` — full API reference for frontend integration

## Frontend integration (short)

- **Auth**
  - Google: obtain an ID token from Google Identity Services, then `POST /api/v1/auth/google` with `{ "id_token": "<credential>" }`.
  - Store returned `token` and send `Authorization: Bearer <token>` on protected routes.
- **Realtime**
  - `GET /api/v1/ws?match_id=<uuid>&token=<jwt>` (browsers cannot set auth headers on WebSockets reliably; use `token` query param).
- **Friend battles**
  - `POST /api/v1/friend-battles` → share code / join path → guest `POST /api/v1/friend-battles/:code/join`.

## Notes / MVP limitations

- **Matchmaking queue is in-memory** (resets on restart; not safe across multiple API replicas without redesign).
- **Friend invite codes** are stored in Postgres; matchmaking queue is not.
- **Uploaded files** are stored on local disk (`./uploads`) and served by Gin static middleware.

