# EncomDB — Session Memory

Read this first if you're resuming work on the project.

**Working dir**: `D:\codezzz\Claude\EncomDB\`

**Status**: v0.1.0 shipped and running on laptop at `http://localhost:8090`. Termux binary built at `bin/encomdb-android-arm64` (25 MB), not yet deployed.

## What it is

Self-hosted SQL database platform on top of PocketBase v0.39. One Go binary. Uses `modernc.org/sqlite` (pure Go, no CGO) so it cross-compiles anywhere Go runs.

The one product loop:
1. Sign into `/dashboard` with a PocketBase superuser account.
2. Click "+ New database" → the server creates a fresh SQLite file under `pb_data/encom_dbs/<name>.sqlite` and mints an API key.
3. Copy the endpoint + key + curl snippet from the "Connect" modal.
4. Your project talks to it via `POST /api/encom/dbs/<name>/sql`.

## File map

```
EncomDB/
├── go.mod                    module github.com/11fsociety/encomdb
├── cmd/encomdb/main.go       PB bootstrap, hooks EnsureCollections, mounts /api/encom/*, serves /dashboard
├── internal/
│   ├── dbs/
│   │   ├── collections.go    creates encom_dbs collection on startup
│   │   ├── manager.go        NewManager, Create/Delete/Open, ConnectionInfo, per-DB SQLite pool
│   │   ├── routes.go         list/create/get/delete + POST /sql runner
│   │   └── time.go           timeNowMs helper
│   └── ui/
│       └── dashboard.go      single embedded HTML page (Tailwind CDN + fetch)
├── scripts/
│   ├── install-termux.sh     on-phone: pkg install golang → go build → boot script
│   └── build-termux.sh       cross-compile from laptop: GOOS=android GOARCH=arm64
├── configs/                  (empty — PB stores config in pb_data)
├── pb_public/                (empty — reserved for future static assets)
├── bin/
│   ├── encomdb.exe                 Windows (33 MB unstripped)
│   └── encomdb-android-arm64       Android ARM64 stripped (25 MB)
└── README.md
```

Note: `pb_data/` is created at runtime and gitignored.

## Locked design decisions

1. **PocketBase is a dependency, not a fork.** All auth, admin UI, JWT, collections handled by PB. We add hooks + one collection + one router group.
2. **Custom API lives at `/api/encom/*`**. PB owns `/api/*` for everything else and `/_/` for admin.
3. **Superuser authentication for DB CRUD.** The dashboard uses `POST /api/admins/auth-with-password` (wait — v0.39 renamed this to superuser; dashboard hits `/api/admins/auth-with-password` which is deprecated. **See "Known bugs" below.**)
4. **API-key authorization for SQL runs.** Format `encomdb_<48 hex chars>`. Compared with a plain equality check today — not constant-time. Fine for LAN, tighten before public exposure.
5. **One SQLite file per database.** WAL mode, `synchronous=NORMAL`, `busy_timeout=5s`, `foreign_keys=1`, `SetMaxOpenConns(1)`. All the same knobs.
6. **Result set cap**: 10,000 rows per query. Beyond that returns 400.
7. **Reads vs writes split by first-word check**: `SELECT/WITH/PRAGMA/EXPLAIN` → `QueryContext`, everything else → `ExecContext`. Not perfect (a `WITH ... UPDATE` will misroute), acceptable for V1.

## API surface

| Method | Path | Auth | What |
|---|---|---|---|
| GET  | `/api/encom/dbs`         | Superuser | list DBs w/ connection info |
| POST | `/api/encom/dbs`         | Superuser | create DB (body `{name, description}`) |
| GET  | `/api/encom/dbs/{name}`  | Superuser | get one DB's connection info |
| DELETE | `/api/encom/dbs/{name}` | Superuser | delete DB (SQLite file + WAL/SHM + metadata row) |
| POST | `/api/encom/dbs/{name}/sql` | Bearer api_key | run SQL — body `{sql, args?}` |

Plus PB's native surface:
- `/_/` — PB admin
- `/api/health`, `/api/collections/*`, `/api/realtime`, `/api/backups/*`, `/api/logs/*`, etc.
- `/dashboard` — our custom SPA

## Data at rest

```
pb_data/
├── data.db          PB: superusers, collections, encom_dbs rows
├── data.db-wal
├── data.db-shm
├── storage/         PB file uploads (unused today)
├── backups/         PB backup snapshots (unused today)
├── logs.db          PB request/system log
└── encom_dbs/
    ├── my_project.sqlite
    ├── my_project.sqlite-wal
    └── my_project.sqlite-shm
```

## Cloudflare Tunnel (built in)

`internal/tunnel/tunnel.go` runs `cloudflared tunnel --url http://<addr>` as a supervised child process. On startup the server looks for `cloudflared` in PATH, `~/bin/`, `~/.local/bin/`, or Termux prefix. If found, it spawns the tunnel, scrapes the URL from stdout/stderr, publishes it via `dbs.Manager.SetTunnelURL()`, and includes it in every `ConnectionInfo` response so the dashboard shows the public URL as the endpoint. If `cloudflared` is missing, we log "LAN only" and continue.

Restart backoff: 3s, doubling to 60s max. Resets to 3s if the child stayed alive >30s.

Env knobs:
- `ENCOMDB_TUNNEL=0` disables the tunnel entirely.
- No `cloudflared` in PATH → automatic LAN-only mode.

Dashboard shows a green badge in the header: `Public: <host>.trycloudflare.com` when a URL is known, or `LAN only` otherwise. Polls `/api/encom/tunnel` every 5s.

**Quick tunnel URLs change on restart.** Documented in README. Switching to a named tunnel (needs a domain on Cloudflare DNS) makes it permanent — one config change.

## Default superuser (seeded on boot)

`internal/dbs/bootstrap.go` seeds a superuser on every startup:

- Email: `asmitdash44@gmail.com`
- Password: `asmitdash44`

Override via env: `ENCOMDB_ADMIN_EMAIL`, `ENCOMDB_ADMIN_PASSWORD`.

The seed is upsert: if the record already exists, the password is refreshed on every boot. This means restarting always yields a known-good login. Downside: someone with filesystem access can't change the password persistently — they'd have to change the env or the constant. Fine for personal use; document if you ever ship to others.

## Known bugs / rough edges

1. ~~Dashboard login endpoint uses PB's deprecated admin path.~~ **FIXED** — dashboard now hits `/api/collections/_superusers/auth-with-password`.
2. **API key comparison uses `==`, not `subtle.ConstantTimeCompare`.** Timing attack possible on the SQL runner. Fix before public exposure.
3. **`isRead` heuristic** in `routes.go` will misclassify `WITH ... UPDATE`. Rare in practice; fix by looking for the first non-CTE verb.
4. **No SQL statement allowlist / denylist.** `DROP TABLE`, `ATTACH DATABASE`, and file-write pragmas are all callable. Owner of the API key can nuke their own DB. That's arguably the right behaviour, but document it.
5. **No per-DB size limit or quota.** A rogue client can fill the disk.
6. **No metrics on the SQL runner path.** Add histogram once we care about performance.
7. **modernc/sqlite version drift warning** on startup: we're on v1.56 but PB tested against v1.55. Harmless so far. Pin explicitly if it bites.

## How to run

**Laptop (Windows):**
```powershell
bin\encomdb.exe serve --http=0.0.0.0:8090
# then http://localhost:8090/_/  (create superuser)
# then http://localhost:8090/dashboard  (make DBs)
```

**Termux (from laptop):**
```bash
bash scripts/build-termux.sh
# push bin/encomdb-android-arm64 to phone + pb_data/
adb push bin/encomdb-android-arm64 /sdcard/encomdb
# on phone:
mv /sdcard/encomdb ~/encomdb/bin/encomdb
chmod +x ~/encomdb/bin/encomdb
cd ~/encomdb && ./bin/encomdb serve --http=0.0.0.0:8090
```

**Termux (git clone flow):**
```bash
pkg install -y git
git clone https://github.com/11fsociety/encomdb ~/encomdb
cd ~/encomdb && bash scripts/install-termux.sh
```
(assumes the repo is public at `github.com/11fsociety/encomdb` — private repo works too with a PAT)

## Smoke check commands

```bash
curl http://localhost:8090/api/health
# {"message":"API is healthy.","code":200,"data":{}}

curl http://localhost:8090/dashboard -o /dev/null -w "%{http_code}\n"
# 200

curl http://localhost:8090/api/encom/dbs
# 401 (correct — no auth)

# After signing into /dashboard, use the browser to:
# 1. click "+ New database", name it "test"
# 2. click "Connect" and copy the endpoint + key
# 3. paste into a terminal and try:
curl -X POST "http://localhost:8090/api/encom/dbs/test/sql" \
  -H "Authorization: Bearer encomdb_<key>" \
  -H "Content-Type: application/json" \
  -d '{"sql":"CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)"}'
```

## Next moves (pick one, small)

1. **Fix the two bugs above.** 20 min.
2. **Add `POST /api/encom/dbs/{name}/backup`** returning a tar.gz snapshot. 30 min.
3. **Add SDK packages** (Go, TypeScript). 1 hr.
4. **Deploy on the phone.** Do the ops-doc walkthrough from `README.md`, keep it running 24h, watch it survive an idle-doze.
5. **Add a second DB "mode"** (KV or time-series). Only after the SQL flow is solid.
