# EncomPortal / RocketDB — Session Memory

**Read this file top to bottom before touching anything.** It's the handoff from previous sessions.

**Working dir**: `D:\codezzz\Claude\EncomDB\`
**Repo**: `github.com/11fsociety/encomdb` (public, live)
**Owner**: Asmit Dash

---

## 1. What this is (one paragraph)

Self-hosted backend platform running on Asmit's Redmi phone in Termux. The **product** is called **EncomPortal**. The **database module** is called **RocketDB**. Built on top of PocketBase v0.39 (imported as go.mod dep, aliased `pb`). A Vercel-hosted static dashboard at `https://encomportal.vercel.app` controls the phone via a serveo.net reverse tunnel; the phone posts its rotating tunnel URL to a Vercel serverless function every 60s so the portal always finds it.

**Working today**: RocketDB module. Click "+ New database" on the portal → SQLite file provisioned on the phone → get an endpoint + API key → run any SQL over HTTP from anywhere.

---

## 2. Locked decisions (do not re-litigate)

1. **PocketBase is a go.mod dep**, aliased as `pb`. Never fork it.
2. **Serveo, not cloudflared.** Free, no ToS on media, uses ssh already in Termux.
3. **In-memory Vercel state** for the tunnel URL. Upstash KV was tried and its Vercel-injected credentials were dead (`WRONGPASS`); walked away from external DB.
4. **Public POST** to `/api/tunnel` (no bearer token) because Vercel CLI's `--value` flag silently stores empty strings and we couldn't reliably set the env var. Documented "if you ever want to lock it, set `ENCOMDB_TUNNEL_REGISTRY_TOKEN` and the code enforces it."
5. **No Hosting via Coolify** — Docker requires Linux namespaces stock Android doesn't expose. Deferred to a Termux child-process supervisor when we get to it.
6. **Auth model = Option B**: multi-tenant Clerk-style (each of Asmit's projects gets its own "app" in EncomPortal with its own users/JWT). NOT just "reuse PB's users collection."
7. **Streaming = catalog only.** Bytes flow from Google Drive to the user's VLC via `vlc://` URLs. Never through the phone's tunnel.
8. **Storage = Google Drive only.** No local/SD-card providers.

---

## 3. Architecture, live today

```
User (browser, anywhere)
  ↓
https://encomportal.vercel.app  (Vercel static SPA + /api/tunnel)
  │  reads current phone URL from in-memory var
  ↓
https://<hash>.serveousercontent.com  (serveo reverse tunnel)
  ↓
Redmi phone in Termux, encomdb :8090
  ├── PocketBase (users, admin at /_/, JWT)
  ├── /api/rocketdb/*  (project CRUD + SQL runner)
  ├── /api/rocketdb/tunnel  (current tunnel URL)
  ├── /dashboard  (fallback on-phone SPA, same as Vercel one)
  ├── serveo.net supervisor  (ssh -R, backoff, restart on drop)
  └── registry poster  (POSTs to encomportal.vercel.app every 60s)

Phone stores:
  pb_data/data.db          — PB internal (users, rocketdb_projects registry)
  pb_data/rocketdb/*.sqlite — one file per managed database
```

**Data flow when portal shows a phone URL:**

1. Phone starts encomdb.
2. Encomdb spawns `ssh -R 80:127.0.0.1:8090 serveo.net`.
3. Encomdb scrapes ssh's stdout for `https://xxx.(serveo.net|serveousercontent.com)`.
4. Encomdb POSTs `{url}` to `https://encomportal.vercel.app/api/tunnel`.
5. Encomdb re-POSTs the same URL every 60s (keep-alive; Vercel functions cold-start).
6. Portal SPA polls `/api/tunnel` every 10s → gets the URL → renders badge + points sign-in form at the phone.

---

## 4. Repo layout (short)

```
EncomDB/
├── go.mod                     module github.com/11fsociety/encomdb
├── cmd/encomdb/main.go        PB bootstrap, wires tunnel, mounts routes
├── internal/
│   ├── rocketdb/              collection + manager + REST handlers
│   ├── tunnel/                serveo supervisor + registry poster
│   └── ui/                    single-file SPA served at /dashboard
├── dashboard/                 Vercel-deployed SPA (fixed public URL)
│   ├── index.html             mirror of internal/ui/ but talks to portal /api/tunnel first
│   ├── api/tunnel.js          serverless — GET/POST current URL (in-memory)
│   ├── package.json
│   └── vercel.json
├── scripts/
│   ├── install-termux.sh      one-line curl-to-sh installer (defaults to encomportal.vercel.app)
│   └── build-termux.sh        cross-compile from a laptop
└── bin/
    ├── encomdb.exe            Windows dev build
    └── encomdb-android-arm64  Termux target (~25 MB stripped)
```

---

## 5. What SHIPPED today (2026-08-05)

### Session 1 (morning)
- **RocketDB module** — full CRUD, per-DB SQLite files, SQL-over-HTTP with API keys, 10k-row cap, dashboard UI, connection modal with copyable curl.
- **Serveo tunnel** with supervisor, 3s→60s backoff, matches both `serveo.net` and `serveousercontent.com`.
- **Vercel dashboard** at `https://encomportal.vercel.app` — fixed URL, dynamic phone URL.
- **One-line Termux installer**: `curl -fsSL https://raw.githubusercontent.com/11fsociety/encomdb/main/scripts/install-termux.sh | sh`.
- **Default admin seeded on boot**: `asmitdash44@gmail.com` / `asmitdash44` (overridable via env).
- **60s keep-alive** so Vercel cold starts recover in under a minute.

### Session 2 (afternoon) - Auth v0.2.0
- **Multi-tenant Auth module** at `internal/auth/`, Clerk-style. `auth_apps` collection + flat `auth_app_users` collection with composite unique `(app_id, email)`. Same email can live in multiple apps.
- **bcrypt password hashing** (cost 10). `secret_key` bcrypt-hashed and shown ONCE at create/rotate.
- **HS256 JWTs** signed with per-app `jwt_secret`. Default TTL 168 h. Cross-app token replay tested - rejected.
- **13 new endpoints** under `/api/auth/*`: apps CRUD, rotate-secret, users list/patch/delete, public signup/login/me.
- **Dashboard rewritten with tabs**: RocketDB / Auth / Storage / Streaming / Hosting. Tabs are one file (both `dashboard/index.html` Vercel + `internal/ui/dashboard.go` on-phone stay in sync). Auth tab: apps list + create modal + Manage modal with pub key, rotate-to-reveal secret, curl hints, users table with disable/delete.
- **Live-verified** via Playwright end-to-end on `localhost:8096`: login → Auth tab → create app → signup user → user shows in Manage modal.

**Verified end-to-end**: Portal → phone SQL query returns in 1ms round-trip from anywhere on the internet. Asmit confirmed working via browser.

---

## 6. Module queue for future sessions

**Locked, in this order:**

- **Session 2 -> Auth (multi-tenant, Clerk-style)** - **DONE 2026-08-05 v0.2.0.**

- **Session 3 → Google Drive Storage**
  - OAuth2 flow (Vercel URL is fixed, so OAuth redirect just works there).
  - Refresh token stored in PB encrypted.
  - `GET /api/storage/drive/list?folder=…` etc.
  - Portal tab: pick a Drive folder to sync.
  - ~1 session.

- **Session 4 → Streaming (catalog only)**
  - Depends on Storage from Session 3.
  - Scans a Drive folder → stores titles + thumbnails in a `streaming_items` collection.
  - Renders Netflix-style grid.
  - Click → `vlc://https://drive.google.com/uc?id=<id>` opens VLC on the client.
  - ~0.5 session.

- **Session 5 → Hosting (Termux child-process supervisor)**
  - Coolify DEAD (needs Docker/root — impossible on stock Android).
  - Instead: paste a git URL + start command → EncomDB clones the repo into `~/encomdb/hosted/<name>/`, runs `<start-cmd>` as a child, captures stdout+stderr, auto-restarts on crash.
  - Portal tab: deployments list, restart button, log tail.
  - ~2 sessions.

**Dropped from earlier plans:**
- Automation module (cron/webhooks) — not in Asmit's priorities.
- Monitoring module — PB has enough built-in.
- Logging viewer — PB admin `/_/` has one.

---

## 7. Known limitations / gotchas

1. **serveo URL rotates every restart.** Portal keep-alive handles this within 60s. If you want a permanent URL, buy a domain + move to a named cloudflared tunnel later.
2. **Serveo occasional downtime.** Community project, no SLA. If it dies, swap providers — `internal/tunnel/tunnel.go` is one file.
3. **Vercel serverless in-memory store loses state on cold start.** Phone re-posts every 60s so recovery is bounded.
4. **API key comparison** uses `==` not `subtle.ConstantTimeCompare`. Fine for personal use, tighten before ever making the phone URL public.
5. **`ENCOMDB_TUNNEL_REGISTRY_TOKEN` env var support is broken end-to-end** — Vercel CLI doesn't actually store the value from `--value` (silently sets empty). The registry POST is currently unauthenticated. Not a functional issue for Asmit's use, but a security consideration.
6. **Argon2id on Snapdragon 720G** takes 300-800ms per PB login verify. Acceptable.
7. **Termux single-tab mode** — Asmit's phone only has one tab. Use `Ctrl+Z` + `bg` if you need to run something else without killing the server. Or `pkill -f 'bin/encomdb'` then restart.

---

## 8. How to resume next session

### First 60 seconds

```bash
cd D:/codezzz/Claude/EncomDB
git status                                     # should be clean, at 5518c88 or later on main
curl -s https://encomportal.vercel.app/api/tunnel  # confirm phone is registered
```

### If the phone shows offline

Ask Asmit to Ctrl+C the encomdb terminal on his phone and run:

```bash
cd ~/encomdb && ENCOMDB_TUNNEL_REGISTRY_URL=https://encomportal.vercel.app/api/tunnel ./bin/encomdb serve --http=0.0.0.0:8090
```

Wait for `[tunnel/registry] registered ...` line.

### Then start Session 2 (Auth)

Design plan:
- New PocketBase collection `apps` (id, name, owner_user_id, api_public_key, api_secret_hash).
- New collection template `<app_id>_users` (dynamic — one per app).
- Routes:
  - `POST /api/apps` (superuser) — create app
  - `GET /api/apps` (superuser) — list
  - `POST /api/apps/{id}/signup {email,password}` — public app-user signup
  - `POST /api/apps/{id}/login {email,password}` → returns JWT
  - `GET /api/apps/{id}/me` (JWT) — current app-user
- Portal tab **"Auth"**: list apps, create app, per-app: users table + API key display.

### Environment

- **Go**: 1.26.5, at `/c/Program Files/Go/bin/go` on Windows. Add to PATH: `export PATH="/c/Program Files/Go/bin:$PATH"`.
- **Vercel CLI**: authenticated as `asmitdash44-5066`, project = `encomportal`.
- **gh CLI**: two accounts (`asmitdash` + `11fsociety`). This repo is `11fsociety`. Switch with `gh auth switch -h github.com -u 11fsociety` before pushing.

### Rules Claude Code must honor

- **HARD RULE**: writes/edits only inside `D:\codezzz\Claude\` unless Asmit says otherwise mid-turn.
- **HARD RULE**: no em-dashes (`—`) in any file. Use `-`.
- **HARD RULE**: on `git push` / `vercel --prod` / any ship command, kenway hook fires and demands confirmation. Prefix retry with `KENWAY_ASKED=1`.
- **HARD RULE**: check `AGENT_CHAT.md` in the workspace root for cross-session context BEFORE answering.
- **HARD RULE**: check user memory `MEMORY.md` at `C:\Users\Asmit Dash\.claude\projects\d--codezzz-Claude\memory\MEMORY.md`.
- **Never leave the repo in a broken state.** Every phase compiles, tests pass (if any), pushed to `main`.
- **When making decisions with tradeoffs**: give a recommendation, don't dump options. Push back if the plan is wrong.

---

## 9. Recent commits on `main`

```
(pending) - Session 2: multi-tenant Auth v0.2.0 (apps + per-app users + JWT + Auth tab)
5518c88 — Match serveousercontent.com URLs (regex fix for serveo domain rotation)
ddc92a8 — In-memory tunnel registry + 60s keep-alive
c49c21c — Portal: default registry URL, drop token requirement
ad17c37 — Migrate legacy encom_dbs collection and rename index
a83fa31 — Rebrand: EncomPortal + RocketDB + Vercel controller
879286e — Swap tunnel provider from cloudflared to serveo.net
c44e88f — Fix cloudflared exit 1 by rewriting 0.0.0.0 to 127.0.0.1
9c6c40b — Initial commit: EncomDB v0.1.0
```

---

## 10. If Asmit asks "what did we do today"

Started with a modular self-hosted backend platform vision, iterated through three scope prunes as the design got clearer, landed on: **RocketDB (per-project SQLite backends) + serveo tunnel + Vercel-hosted permanent dashboard**. Ships from a Redmi in Termux to anywhere in the world via a one-line curl. Real DBs, real SQL, real HTTP endpoints. First working demo hit 1ms round-trip SQL through the tunnel by end of day.

Next up: multi-tenant auth as-a-service (like Clerk, but on his phone).
