# EncomPortal — Vercel-hosted dashboard for EncomDB

Fixed public URL. Your phone rotates tunnel URLs; the portal always stays at `https://encomportal.vercel.app`. The dashboard fetches the current phone URL on load and talks CORS to it directly.

## First-time deploy

```bash
cd dashboard/
npm i -g vercel      # if you don't have it
vercel                # first run: link to a new project called "encomportal"
```

Then create a **Vercel KV store** in the Vercel dashboard (Storage tab), attach it to this project. Vercel auto-injects `KV_*` env vars.

Set one more env var:

```
ENCOMDB_TUNNEL_REGISTRY_TOKEN = <random 32-char hex>
```

Generate with `openssl rand -hex 16` and paste it in Vercel's Environment Variables page.

Redeploy: `vercel --prod`.

## Wire the phone side

On your phone, either:

```bash
# option 1: one-off run with env inline
ENCOMDB_TUNNEL_REGISTRY_URL=https://encomportal.vercel.app/api/tunnel \
ENCOMDB_TUNNEL_REGISTRY_TOKEN=<same-token> \
  ./bin/encomdb serve --http=0.0.0.0:8090

# option 2: bake into the install (Termux:Boot picks it up on reboot)
ENCOMDB_TUNNEL_REGISTRY_URL=https://encomportal.vercel.app/api/tunnel \
ENCOMDB_TUNNEL_REGISTRY_TOKEN=<same-token> \
  bash scripts/install-termux.sh
```

Now every time serveo hands the phone a new URL, the phone POSTs it to `https://encomportal.vercel.app/api/tunnel`. The dashboard picks it up within 10 seconds.

## Endpoints

- `GET /api/tunnel` — returns `{ url, updated_at }` — public, no auth (the URL is the point).
- `POST /api/tunnel` — sets `{ url }` in Vercel KV. Requires `Authorization: Bearer <token>`.
