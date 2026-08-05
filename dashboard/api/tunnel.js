// EncomPortal — tunnel registry endpoint.
//
// GET  /api/tunnel                    → { url, updated_at }
// POST /api/tunnel  { url }           → stores the current phone tunnel URL
//                                        requires Authorization: Bearer <ENCOMDB_TUNNEL_REGISTRY_TOKEN>
//
// Talks to Upstash Redis directly over their REST API (avoids the
// @vercel/kv wrapper — one less dep to explain).

const KV_KEY = 'encomdb:tunnel';

function kvURL() {
  return (process.env.KV_REST_API_URL || '').replace(/\/+$/, '');
}
function kvToken() {
  return process.env.KV_REST_API_TOKEN || '';
}

async function kvGet(key) {
  const res = await fetch(`${kvURL()}/get/${encodeURIComponent(key)}`, {
    headers: { Authorization: `Bearer ${kvToken()}` },
  });
  if (!res.ok) {
    throw new Error(`kv get ${res.status}: ${await res.text()}`);
  }
  const j = await res.json();
  return j?.result ?? null;
}

async function kvSet(key, value) {
  const res = await fetch(`${kvURL()}/set/${encodeURIComponent(key)}`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${kvToken()}`,
      'Content-Type': 'text/plain',
    },
    body: typeof value === 'string' ? value : JSON.stringify(value),
  });
  if (!res.ok) {
    throw new Error(`kv set ${res.status}: ${await res.text()}`);
  }
  return true;
}

export default async function handler(req, res) {
  if (req.method === 'OPTIONS') return res.status(204).end();

  if (!kvURL() || !kvToken()) {
    return res.status(200).json({
      url: null,
      updated_at: null,
      error: 'kv-not-configured',
      detail: 'KV_REST_API_URL and KV_REST_API_TOKEN must be set',
    });
  }

  if (req.method === 'GET') {
    try {
      const raw = await kvGet(KV_KEY);
      if (!raw) {
        return res.status(200).json({ url: null, updated_at: null, note: 'no tunnel registered yet' });
      }
      let record;
      try { record = typeof raw === 'string' ? JSON.parse(raw) : raw; } catch { record = null; }
      if (!record) {
        return res.status(200).json({ url: null, updated_at: null, note: 'stored value malformed' });
      }
      return res.status(200).json(record);
    } catch (err) {
      return res.status(200).json({
        url: null,
        updated_at: null,
        error: 'kv-read-failed',
        detail: String(err?.message || err),
      });
    }
  }

  if (req.method === 'POST') {
    // Optional bearer-token guard. If ENCOMDB_TUNNEL_REGISTRY_TOKEN is set,
    // require it. Otherwise the endpoint is open — fine for personal use
    // where the domain is unadvertised and the payload is just a URL.
    const expected = (process.env.ENCOMDB_TUNNEL_REGISTRY_TOKEN || '').trim();
    if (expected) {
      const auth = req.headers.authorization || '';
      if (!auth.toLowerCase().startsWith('bearer ') || auth.slice(7).trim() !== expected) {
        return res.status(401).json({ error: 'invalid token' });
      }
    }

    let body = req.body;
    if (typeof body === 'string') {
      try { body = JSON.parse(body); } catch { body = null; }
    }
    const url = body && typeof body.url === 'string' ? body.url.trim() : '';
    if (!url) return res.status(400).json({ error: 'url required' });
    if (!/^https?:\/\//i.test(url)) return res.status(400).json({ error: 'url must start with http:// or https://' });

    const record = { url, updated_at: new Date().toISOString() };
    try {
      await kvSet(KV_KEY, record);
    } catch (err) {
      return res.status(500).json({ error: 'kv-write-failed', detail: String(err?.message || err) });
    }
    return res.status(200).json({ ok: true, ...record });
  }

  return res.status(405).json({ error: 'method not allowed' });
}
