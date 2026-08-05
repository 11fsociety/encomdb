# EncomPortal

**EncomPortal** is a self-hosted backend platform that runs on your phone. **RocketDB** is its flagship database module — click "New database" in the dashboard and get a managed SQLite backend with a real endpoint + API key. All of it runs inside Termux, no root, no cloud fees.

- **RocketDB** — one SQLite file per project, SQL over HTTP.
- **serveo.net tunnel** — free public URL to reach your phone.
- **EncomPortal dashboard** — a fixed URL on Vercel (`https://encomportal.vercel.app`) that always finds your phone even when its tunnel URL rotates.

## One-command Termux install

Basic (LAN + random tunnel URL):

```bash
curl -fsSL https://raw.githubusercontent.com/11fsociety/encomdb/main/scripts/install-termux.sh | sh
```

With the portal wired (recommended):

```bash
ENCOMDB_TUNNEL_REGISTRY_URL=https://encomportal.vercel.app/api/tunnel \
ENCOMDB_TUNNEL_REGISTRY_TOKEN=<your-secret> \
  bash -c "curl -fsSL https://raw.githubusercontent.com/11fsociety/encomdb/main/scripts/install-termux.sh | sh"
```

The install script:
1. `pkg install git golang openssh termux-api`
2. Clones the repo to `~/encomdb`
3. Builds the binary from source
4. Installs a Termux:Boot supervisor
5. Starts the server + serveo.net tunnel
6. If registry env vars are set, phones home the URL to your Vercel portal

Wait 2-5 min. When you see `[tunnel] PUBLIC URL: https://<random>.serveo.net`, that's your phone.

## First-time defaults

- Admin email: `asmitdash44@gmail.com`
- Admin password: `asmitdash44`

Override with `ENCOMDB_ADMIN_EMAIL` / `ENCOMDB_ADMIN_PASSWORD`.

## Deploying the EncomPortal dashboard on Vercel

See [dashboard/README.md](./dashboard/README.md) for the 3-minute setup. Fixed URL forever, free forever, tunnel URLs auto-tracked.

## Using a RocketDB database

1. Open the portal (`https://encomportal.vercel.app`) or the on-phone dashboard (`http://<tunnel>/dashboard`).
2. Sign in with the admin credentials.
3. Click **"+ New database"**, give it a name.
4. Click **"Connect"** — copy the endpoint, API key, or the ready-made curl.

```bash
curl -X POST https://<phone>/api/rocketdb/dbs/my_project/sql \
  -H "Authorization: Bearer encomdb_<key>" \
  -H "Content-Type: application/json" \
  -d '{"sql":"CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT)"}'
```

Reads return `{columns, rows, duration_ms}`. Writes return `{rows_affected, duration_ms}`. Result sets are capped at 10,000 rows per query.

## API surface

Admin (superuser JWT):

| Method | Path | What |
|---|---|---|
| POST | `/api/rocketdb/dbs` | create DB |
| GET  | `/api/rocketdb/dbs` | list DBs |
| GET  | `/api/rocketdb/dbs/{name}` | one DB's connection info |
| DELETE | `/api/rocketdb/dbs/{name}` | delete DB |
| GET  | `/api/rocketdb/tunnel` | current tunnel URL |

Runtime (Bearer API key):

| Method | Path | What |
|---|---|---|
| POST | `/api/rocketdb/dbs/{name}/sql` | run SQL |

## Config

- `ENCOMDB_TUNNEL=0` — disable tunnel (LAN only)
- `ENCOMDB_TUNNEL_SUBDOMAIN=foo` — request `https://foo.serveo.net`
- `ENCOMDB_TUNNEL_REGISTRY_URL` — post current tunnel URL to your Vercel portal
- `ENCOMDB_TUNNEL_REGISTRY_TOKEN` — shared secret with the portal
- `ENCOMDB_ADMIN_EMAIL`, `ENCOMDB_ADMIN_PASSWORD` — override seeded superuser
- `ENCOMDB_PUBLIC_HOST` — fallback for connection strings when no tunnel is up

## Where things live

- `pb_data/data.db` — internal state DB (superusers, RocketDB project registry).
- `pb_data/rocketdb/<name>.sqlite` — one file per managed database.
- `bin/encomdb` — the server binary.

## License

MIT.
