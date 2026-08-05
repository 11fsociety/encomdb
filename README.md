# EncomDB

Self-hosted SQL database platform. One Go binary. Runs on your laptop, on a Raspberry Pi, or on a phone in Termux. Built on top of [PocketBase](https://pocketbase.io).

- Click **"New database"** in the dashboard → gets you a fresh SQLite DB + API key + connection string.
- Every managed DB is one SQLite file. Talk to it with SQL over HTTP.
- Publicly reachable out of the box via **Cloudflare Tunnel** (free, no domain required).
- No Docker, no root, no Postgres server.

## One-command Termux install

```bash
curl -fsSL https://raw.githubusercontent.com/11fsociety/encomdb/main/scripts/install-termux.sh | sh
```

This will:
1. `pkg install git golang wget termux-api`
2. Clone the repo to `~/encomdb`
3. Download `cloudflared` to `~/bin/`
4. Build the binary
5. Install a `Termux:Boot` supervisor so it survives reboots
6. Start the server + tunnel
7. Print the public URL

Wait 2-5 min (Go build is slow on-phone). When it says `[tunnel] PUBLIC URL: https://<random>.trycloudflare.com`, that's your endpoint.

Then open the URL and go to `/dashboard`. Sign in:

- Email: `asmitdash44@gmail.com`
- Password: `asmitdash44`

(Override with `ENCOMDB_ADMIN_EMAIL` / `ENCOMDB_ADMIN_PASSWORD`.)

## Laptop dev

```bash
go build -o bin/encomdb ./cmd/encomdb
./bin/encomdb serve --http=0.0.0.0:8090
```

Then [http://localhost:8090/dashboard](http://localhost:8090/dashboard). Tunnel is disabled automatically if `cloudflared` isn't installed locally — the server logs `LAN only`.

## Using a database

1. Sign into `/dashboard`.
2. Click **"+ New database"**, give it a name (lowercase letters/digits/`_`/`-`, 3-40 chars).
3. Click **"Connect"** on the card. Copy the endpoint, API key, or the curl snippet — the endpoint already points at your public tunnel URL.
4. Talk to it:

```bash
curl -X POST https://<tunnel>.trycloudflare.com/api/encom/dbs/my_project/sql \
  -H "Authorization: Bearer encomdb_<key>" \
  -H "Content-Type: application/json" \
  -d '{"sql":"CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT)"}'

curl -X POST https://<tunnel>.trycloudflare.com/api/encom/dbs/my_project/sql \
  -H "Authorization: Bearer encomdb_<key>" \
  -H "Content-Type: application/json" \
  -d '{"sql":"INSERT INTO notes(body) VALUES (?)","args":["hi"]}'
```

Reads return `{columns, rows, duration_ms}`. Writes return `{rows_affected, duration_ms}`. Result sets capped at 10,000 rows.

## What the API looks like

Admin (PB superuser JWT required):

| Method | Path | What |
|---|---|---|
| POST | `/api/encom/dbs` | create DB |
| GET  | `/api/encom/dbs` | list DBs |
| GET  | `/api/encom/dbs/{name}` | one DB's connection info |
| DELETE | `/api/encom/dbs/{name}` | delete DB |
| GET  | `/api/encom/tunnel` | current public URL |

Runtime (Bearer `<api_key>`):

| Method | Path | What |
|---|---|---|
| POST | `/api/encom/dbs/{name}/sql` | run SQL |

## Config knobs

- `ENCOMDB_TUNNEL=0` — disable Cloudflare Tunnel (LAN only).
- `ENCOMDB_ADMIN_EMAIL`, `ENCOMDB_ADMIN_PASSWORD` — override the seeded superuser.
- `ENCOMDB_PUBLIC_HOST` — override the base URL rendered in connection strings when no tunnel is running.

## Where things live

- `pb_data/data.db` — PocketBase's own DB (users, `encom_dbs` collection).
- `pb_data/encom_dbs/<name>.sqlite` — one file per managed DB.
- `bin/encomdb` — the binary.
- `~/bin/cloudflared` — the tunnel binary (fetched by install script).

## Quick tunnel URL changes on restart

That's how Cloudflare's free "trycloudflare.com" tunnels work — no domain needed, but the URL is not stable across restarts.

Once you own a domain on Cloudflare, we can switch to a named tunnel and the URL becomes permanent. Costs ~₹900/yr for the domain; the tunnel itself stays free.

## License

MIT.
