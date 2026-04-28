# Codeit API — backend reference

Base URL: `http://localhost:8080` (or your host). All JSON APIs are under **`/api/v1`** unless noted.

## Project overview (for frontend)

Codeit backend is a **1v1 coding duel** API. The current user journey is:

1. Register/login to get a JWT.
2. Enter matchmaking for a difficulty (`easy|medium|hard`).
3. Once paired, receive a `match_found` WebSocket event (and HTTP response) containing the match.
4. Open a WebSocket connection with `match_id` for realtime match events.
5. Submit code to Judge0 through backend (`POST /matches/:id/submissions`).
6. When the timer elapses with no further submit, call **`POST /matches/:id/resolve`** so the server can finish by best score (same rules as a post-deadline submit).
7. Receive `submission_received` updates and eventually `match_ended`.
8. Ratings update automatically when match ends.

Frontend should treat backend as source of truth for:

- match lifecycle (`running`/`finished`)
- judge results (`passed_count`)
- final result (`player1|player2|draw`)
- rating changes (visible via profile fetches after match end)
- `world_rank` and `rating_title` on user payloads (derived from rating; no extra DB column)

---

## Frontend integration checklist

### 1) Auth

- Call `POST /auth/login` and store `token`.
- Send `Authorization: Bearer <token>` for all protected HTTP routes.
- Send same header when opening WebSocket.

### 2) Matchmaking screen

- `POST /matchmaking` with selected difficulty.
- Handle:
  - `202 queued` -> show waiting UI
  - `200 matched` -> navigate to match screen
  - `409` -> show conflict reason from `error`
- Also listen for WS `match_found` (user may be on waiting screen while match is found asynchronously).

### 3) Match screen

- Open WS: `GET /api/v1/ws?match_id=<match_id>&token=<jwt>` (browsers cannot set `Authorization` on WebSocket; use `token` query)
- On mount, fetch `GET /matches/:id` to hydrate current state.
- Start countdown using:
  - `started_at` + `duration_seconds`
- On interval or reconnect, re-check `GET /matches/:id` for source-of-truth status.
- When countdown reaches **zero** (server deadline), call **`POST /matches/:id/resolve`** once per client if needed; idempotent if already finished.

### 4) Submitting code

- Call `POST /matches/:id/submissions` with `language` and `code`.
- Do not send pass count; backend computes it from Judge0.
- Update UI from response + WS:
  - `submission_received`
  - `match_ended`

### 5) Result/rating screen

- Use `match.result` + `winner_id` to render winner/draw.
- Fetch both players via `GET /users/:id` to display updated ratings, `world_rank`, and `rating_title`.

### 6) Profile / history

- `GET /me/stats` for headline win/loss/draw.
- `GET /me/matches?limit=&offset=` for paginated past duels (`my_result`, `my_rating_after`, `my_elo_delta`, `opponent_id`, full `match`).

## Environment variables

| Variable | Required | Purpose |
|----------|----------|---------|
| `DATABASE_URL` | Yes | PostgreSQL connection string for `pgxpool` |
| `JWT_SECRET` | Yes | HMAC secret for signing JWTs (app panics if unset) |
| `JUDGE0_BASE_URL` | Yes | Judge0 API base URL (direct Judge0 or RapidAPI endpoint) |
| `JUDGE0_API_KEY` | No | API key/token used for Judge0 auth |
| `JUDGE0_RAPIDAPI_HOST` | No | If set, client sends RapidAPI headers (`X-RapidAPI-Host`, `X-RapidAPI-Key`) |
| `PORT` | No | HTTP listen port (default `8080`) |

## Authentication

Protected routes use **`Authorization: Bearer <jwt>`**.

- JWT is issued on **login**, valid **24 hours** (`internal/auth/jwt.go`).
- User id is stored in claim `user_id` and injected into Gin context by `AuthMiddleware`.

---

## Endpoints

### Health (no prefix)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/health` | No | Liveness: `{ "status": "ok" }` |

---

### Users & auth (`/api/v1`)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/auth/register` | No | Create account |
| `POST` | `/auth/login` | No | Returns JWT + user |
| `GET` | `/users/:id` | No | Public profile (includes `avatar_url`, `world_rank`, `rating_title`) |
| `GET` | `/users/:id/stats` | No | Public match stats for user id (same shape as `/me/stats`) |
| `GET` | `/u/:username` | No | Public profile by username (share link; same shape as `/users/:id`) |
| `GET` | `/leaderboard` | No | Global rating leaderboard (`limit`, `offset` query params) |
| `GET` | `/me` | Yes | Same as public profile for the authenticated user |
| `GET` | `/me/matches` | Yes | Paginated **finished** match history for the current user |
| `GET` | `/me/stats` | Yes | Win / loss / draw counts from finished matches |
| `PATCH` | `/me/avatar` | Yes | Update your avatar URL |
| `POST` | `/me/avatar/upload` | Yes | Upload avatar file and set `avatar_url` automatically |
| `PATCH` | `/users/:id/rating` | Yes | **Manual** rating set (admin/debug; not tied to match flow) |

#### `POST /api/v1/auth/register`

**Body:**

```json
{
  "username": "string",
  "email": "string",
  "password": "string (min 6 chars)"
}
```

**Responses:**

- `201` — created user JSON (password field cleared).
- `400` — invalid input.
- `500` — persistence / server error.

Default **rating** for new users: **1200** (`internal/users/service.go`).

#### `POST /api/v1/auth/login`

**Body:**

```json
{
  "email": "string",
  "password": "string"
}
```

**Responses:**

- `200` — `{ "token": "<jwt>", "user": { ... } }` (password cleared on user).
- `401` — invalid credentials.
- `400` / `500` — as applicable.

#### Rating title (computed from Elo)

Titles update automatically whenever the user’s `rating` changes (no separate DB field). Frontend should read `rating_title` on user objects.

| Rating range | Title |
|--------------|--------|
| 0–999 | Novice |
| 1000–1199 | Apprentice |
| 1200–1399 | Specialist |
| 1400–1599 | Expert |
| 1600–1799 | Master |
| 1800–1999 | Grandmaster |
| 2000+ | Legend |

#### `world_rank` on profiles

`world_rank` is **1-based**: `1 +` count of users with **strictly higher** `rating`. Users with the same rating can share the same rank number.

Returned on: `GET /users/:id`, `GET /me`, `POST /auth/register`, `POST /auth/login` (user payload).

#### Public profile share links (`GET /api/v1/u/:username`)

Use this endpoint to build shareable profile URLs without exposing internal user ids.

- **Path:** `GET /api/v1/u/:username`
- **Auth:** none
- **Response shape:** same as `GET /api/v1/users/:id` (includes `avatar_url`, `rating`, `world_rank`, `rating_title`)
- **Lookup:** case-insensitive match on `username`

**Note:** For best UX, treat usernames as unique. This repo does not enforce a uniqueness constraint on `users.username` yet.

#### `GET /api/v1/leaderboard`

**Auth:** none.

**Query:**

- `limit` — default `50`, max `100`
- `offset` — default `0`

**Response `200`:**

```json
{
  "entries": [
    {
      "world_rank": 1,
      "user_id": "...",
      "username": "alice",
      "avatar_url": "https://...",
      "rating": 1500,
      "title": "Expert"
    }
  ]
}
```

#### `GET /api/v1/me/matches`

**Auth:** required.

**Query:**

- `limit` — default **20**, max **100**
- `offset` — default **0** (non-negative)

Only **`status: finished`** matches where the user is **player1** or **player2** are returned, newest **`ended_at`** first (`NULL` `ended_at` last).

Each row includes **`my_rating_after`** and **`my_elo_delta`** when that match has a stored Elo snapshot (matches finished after the rating-snapshot migration). Older rows omit them (`null` / omitted). The nested **`match`** also includes **`player1_rating_after`**, **`player2_rating_after`**, **`player1_elo_delta`**, **`player2_elo_delta`** when present.

**Response `200`:**

```json
{
  "matches": [
    {
      "match": {
        "id": "...",
        "player1_id": "...",
        "player2_id": "...",
        "problem_id": 1,
        "status": "finished",
        "duration_seconds": 1800,
        "victory_type": "ko",
        "result": "player1",
        "started_at": "...",
        "ended_at": "...",
        "winner_id": "...",
        "created_at": "...",
        "player1_rating_after": 1215,
        "player2_rating_after": 1185,
        "player1_elo_delta": 15,
        "player2_elo_delta": -15
      },
      "opponent_id": "<other user id>",
      "my_result": "win | loss | draw",
      "my_rating_after": 1215,
      "my_elo_delta": 15
    }
  ],
  "total": 42,
  "limit": 20,
  "offset": 0
}
```

#### `GET /api/v1/me/stats`

**Auth:** required.

Aggregates over the same set as history: finished matches where the user participated.

**Response `200`:**

```json
{
  "matches_played": 42,
  "wins": 20,
  "knockout_wins": 8,
  "losses": 18,
  "draws": 4
}
```

#### `PATCH /api/v1/me/avatar`

**Auth:** required.

**Body:**

```json
{
  "avatar_url": "https://example.com/avatar.png"
}
```

`avatar_url` may be an empty string to clear the avatar.

**Response `200`:** updated user profile (same shape as `GET /me`).

#### `POST /api/v1/me/avatar/upload`

**Auth:** required.

**Content-Type:** `multipart/form-data`

**Form field:**

- `avatar` (file)

Accepted file extensions: `.jpg`, `.jpeg`, `.png`, `.webp`, `.gif`  
Max size: `5MB`

**Response `200`:** updated user profile.

For local dev, uploaded files are stored under `./uploads/avatars` and served at:

- `GET /uploads/avatars/<filename>`

Ordering: `rating` descending, then `created_at` ascending. Rank uses SQL `RANK()` so tied ratings share the same `world_rank`.

---

### Problems (`/api/v1`)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/problems` | No | List all problems (with sample test cases only) |
| `GET` | `/problems/:id` | No | Problem by numeric id (samples only) |
| `GET` | `/problems/random?difficulty=...` | No | Random problem for difficulty |

**Random query:** `difficulty` is required by the problems service (empty → `400` invalid difficulty).

**Note:** Full hidden test cases are never returned to clients; they are used server-side for judging (`GetAllTestCasesForJudge`).

---

### Matches (`/api/v1`)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/matches/:id` | No | Match by UUID |
| `GET` | `/matches/active` | Yes | Latest **waiting** or **running** match for the current user |

#### Match JSON shape (high level)

Returned objects include fields such as:

- `id`, `player1_id`, `player2_id`, `problem_id`
- `status`: `waiting` | `running` | `finished`
- `duration_seconds`, `started_at`, `ended_at`, `winner_id`, `created_at`
- `victory_type` (when finished): `ko` | `decision` | `draw`
- **`result`** (computed, not stored): `pending` | `player1` | `player2` | `draw`
  - `finished` + `winner_id == null` → **`draw`**
  - `finished` + winner equals player → **`player1`** or **`player2`**
- **Elo snapshot** (stored when ratings run for that match): `player1_rating_after`, `player2_rating_after`, `player1_elo_delta`, `player2_elo_delta` (omitted when unknown / pre-migration)

Match lifecycle rules (`internal/matches/service.go` + repository):

- **Create (matchmaking path):** match is created already **`running`** with `started_at` set and `duration_seconds` set (default **30 minutes** from matchmaking).
- **`StartMatch`:** only allowed if status is **`waiting`** (legacy / future use).
- **`FinishMatch`:** only from **`running`** → **`finished`**; winner must be one of the two players **or** empty string for a **draw** (`winner_id` stored as SQL NULL).

---

### Matchmaking (`/api/v1`)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/matchmaking` | Yes | Join queue or get matched immediately |
| `DELETE` | `/matchmaking` | Yes | Leave queue (in-memory) |

#### `POST /api/v1/matchmaking`

**Body (optional):**

```json
{
  "difficulty": "easy | medium | hard"
}
```

If `difficulty` is omitted, it defaults to **`easy`**.

**Validation:** only `easy`, `medium`, `hard` (case-insensitive). Anything else → **`400`** `invalid difficulty`.

**Behavior (`internal/matchmaking/service.go`):**

- In-memory **FIFO queue per difficulty** (single-server MVP; not shared across instances).
- If user already has an **active** match (`waiting` or `running`) → **`409`** `user already has an active match`.
- If user already **queued** (any difficulty) → **`409`** `user is already in matchmaking queue`.
- If queue empty for that difficulty → enqueue; response **`202`**:
  ```json
  { "status": "queued", "difficulty": "easy" }
  ```
- If another player was waiting → create match, pick random problem for that difficulty, response **`200`**:
  ```json
  { "status": "matched", "match": { ... } }
  ```

**WebSocket:** on match, both players receive:

```json
{ "type": "match_found", "payload": <match object> }
```

(sent to **user** channels, not match room).

#### `DELETE /api/v1/matchmaking`

**Response:** `{ "left_queue": true | false }`

---

### Submissions (`/api/v1`)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/matches/:id/submissions` | Yes | Submit code for match `:id` (UUID) |
| `POST` | `/matches/:id/resolve` | Yes | After match **deadline**, finish by best `passed_count` (no Judge0); same outcome as post-deadline submit |

#### Request body

```json
{
  "language": "go | python | javascript | java | cpp (aliases supported)",
  "code": "source code string"
}
```

**Trusted judging:** `passed_count` is **not** accepted from the client. The server loads **all** test cases for the match’s `problem_id` and runs each through **Judge0** (`internal/submissions/judge0.go`). A case passes if Judge0 returns **status id `3` (Accepted)**.

Auth header mode:

- If `JUDGE0_RAPIDAPI_HOST` is set: uses `X-RapidAPI-Host` + `X-RapidAPI-Key`.
- Otherwise: uses `X-Auth-Token` when `JUDGE0_API_KEY` is set.

**Supported `language` values (normalized):**

| Request aliases | Judge0 `language_id` |
|-----------------|----------------------|
| `go`, `golang` | 95 |
| `python`, `python3` | 71 |
| `javascript`, `node`, `nodejs` | 63 |
| `java` | 62 |
| `cpp`, `c++` | 54 |

#### Submission rules (`internal/submissions/service.go`)

- Caller must be **`player1_id` or `player2_id`** else **`403`**.
- Match must exist and be **`running`** else **`404`** / **`409`** (`match not found` / `match is not running`).
- After judging, a row is inserted into **`submissions`** (see DB expectations below).

**Match outcome from submissions:**

1. **Before deadline** (`started_at + duration_seconds`):
   - If `passed_count == total_count` (all tests pass) → **immediate win** for submitter; `FinishMatch(matchId, userId)`.
2. **After deadline:**
   - Compare each player’s **best** `passed_count` in this match from DB.
   - Higher → that player wins; equal → **draw** (`FinishMatch` with empty winner).

**HTTP response `201`:**

```json
{
  "submission": {
    "id": "...",
    "match_id": "...",
    "user_id": "...",
    "language": "go",
    "code": "...",
    "passed_count": 3,
    "total_count": 5,
    "status": "judged",
    "submitted_at": "..."
  },
  "match_finished": false,
  "match_result": ""
}
```

When the match ends in the same request, `match_finished` is `true` and `match_result` is set to the computed **`result`** string (`player1` | `player2` | `draw`).

#### `POST /api/v1/matches/:id/resolve`

**Auth:** required. Caller must be **`player1_id` or `player2_id`**.

**Body:** none.

**When to call:** after **`started_at + duration_seconds`** (server time). The UI countdown reaching zero should trigger this if no further submission will run the post-deadline path.

**Behavior:** same best-score / draw logic as **after deadline** on submit (`internal/submissions/service.go` → `finishByBestScore`). Applies **Elo** when this call transitions the match to **finished** (`resolved: true`). If the match was already **finished**, returns **`200`** with `already_finished: true` (idempotent).

**Response `200`:**

```json
{
  "match": { "...": "full match object" },
  "resolved": true,
  "already_finished": false
}
```

- **`resolved: true`** — this request finished the match; a **`match_ended`** WS is broadcast to the match room.
- **`resolved: false`, `already_finished: true`** — match was already finished (e.g. opponent resolved or won earlier).

**Errors:**

- **`400`** — invalid match state for resolve (e.g. missing `started_at` / invalid duration).
- **`403`** — not a participant.
- **`404`** — match not found.
- **`409`** — `match has not expired yet` (before deadline) or `match is not running`.

#### Ratings after match (`internal/ratings`)

When a match transitions to **finished** via **submissions** or **`POST /matches/:id/resolve`**, **Elo** is applied in one DB transaction (`K=32`, clamp **100–3500**). Draw uses score **0.5** for each side relative to **player1** ordering.

If rating update fails, the error is **logged**; the HTTP submission response still succeeds (match result remains committed).

#### WebSocket (same connection model as below)

On submit:

- **`submission_received`** to match room `match_id` (if clients connected with that query):
  - `user_id`, `passed_count`, `total_count`, `submitted_at`
- If match ended (submit or resolve): **`match_ended`** with full **match** payload (includes `result`).

---

### WebSocket (`/api/v1`)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/ws?match_id=<optional>&token=<jwt>` | Yes | Realtime events (JWT via `token` query; `Authorization: Bearer` also works for non-browser clients) |

**Registration (`internal/ws/hub.go`):**

- Client is always keyed by **user id**.
- If `match_id` query is non-empty, client also joins that **match room** for `BroadcastToMatch`.
- Server currently uses WS as **server -> client push** channel; inbound client WS messages are not handled yet.

**Server → client events** (JSON `{ "type": "...", "payload": ... }`):

| `type` | When | `payload` |
|--------|------|-------------|
| `match_found` | Matchmaking paired two users | Full match object |
| `submission_received` | Someone submitted in the match | Summary fields (see above) |
| `match_ended` | Match finished after a submission or **`POST /matches/:id/resolve`** | Full match object |

### Event payload contracts

`match_found`:

```json
{
  "type": "match_found",
  "payload": {
    "id": "match_uuid",
    "player1_id": "user_uuid",
    "player2_id": "user_uuid",
    "problem_id": 1,
    "status": "running",
    "duration_seconds": 1800,
    "started_at": "2026-01-01T10:00:00Z",
    "ended_at": null,
    "winner_id": null,
    "result": "pending",
    "created_at": "2026-01-01T10:00:00Z"
  }
}
```

`submission_received`:

```json
{
  "type": "submission_received",
  "payload": {
    "user_id": "user_uuid",
    "passed_count": 3,
    "total_count": 5,
    "submitted_at": "2026-01-01T10:05:00Z"
  }
}
```

`match_ended`:

```json
{
  "type": "match_ended",
  "payload": {
    "id": "match_uuid",
    "status": "finished",
    "winner_id": null,
    "result": "draw"
  }
}
```

**Note:** `CheckOrigin` currently allows all origins (MVP).

---

## Database tables (expected)

The code assumes PostgreSQL tables including at least:

- **`users`** — id, username, email, password, rating, created_at  
- **`problems`**, **`test_cases`** — as used by `problems` repository  
- **`matches`** — includes **`duration_seconds`**, status, timestamps, nullable **`winner_id`**, optional **`player1_rating_after`**, **`player2_rating_after`**, **`player1_elo_delta`**, **`player2_elo_delta`** (set when Elo is applied for that match)  
- **`submissions`** — id, match_id, user_id, language, code, passed_count, total_count, status, submitted_at  

There is **no migration runner** in this repo; schema must match these expectations.

---

## Architectural summary

| Area | Responsibility |
|------|----------------|
| `internal/users` | Register, login, profiles, manual rating patch |
| `internal/problems` | CRUD-style read/list; hidden tests for judge only |
| `internal/matches` | Match persistence, lifecycle, computed `result`, finished-match history + stats |
| `internal/matchmaking` | In-memory queue + pair + create running match + WS `match_found` |
| `internal/submissions` | Judge0 per test, persist submission, post-deadline resolve, finish match, trigger ratings |
| `internal/ratings` | Post-match Elo updates (transactional) |
| `internal/ws` | Hub, per-user and per-match broadcast |
| `internal/auth` | JWT middleware |

---

## Known MVP limitations

- **Single-instance** matchmaking (in-memory queue resets on restart; not safe across multiple API replicas).
- **Judge0** must be reachable at process start (`JUDGE0_BASE_URL` required).
- **`JWT_SECRET`** must be set or the process will panic on package init for auth.
- Finishing a match **only** via direct DB edits or a future admin path would **not** trigger Elo unless the same rating hook runs. **`POST /matches/:id/resolve`** does trigger ratings when it finishes a running match.

---

## Quick test flow

1. `POST /api/v1/auth/register` → `POST /api/v1/auth/login` → copy `token`.
2. Open two users / two tokens; both `POST /api/v1/matchmaking` with same `difficulty` until `matched`.
3. Both connect `ws://localhost:8080/api/v1/ws?match_id=<match_uuid>&token=<jwt>` (or set Bearer header where supported).
4. `POST /api/v1/matches/<match_uuid>/submissions` with `language` + `code`.
5. `GET /api/v1/matches/<match_uuid>` or `/matches/active` to inspect state and `result`.
