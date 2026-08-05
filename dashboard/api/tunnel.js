// EncomPortal — tunnel registry endpoint (in-memory, no external DB).
//
// GET  /api/tunnel                → { url, updated_at }
// POST /api/tunnel  { url }       → stores the current phone tunnel URL
//
// Storage: process-global variable. Survives while the Vercel serverless
// function stays warm (minutes to hours). Phone keeps re-posting every
// 60s via the tunnel supervisor, so a cold-start reset just means the
// dashboard sees "offline" for up to 60s until the next post.
//
// No auth by default. Set ENCOMDB_TUNNEL_REGISTRY_TOKEN in the Vercel env
// to require Authorization: Bearer <token> on POST.

globalThis.__encomdb_tunnel = globalThis.__encomdb_tunnel || { url: null, updated_at: null };

export default async function handler(req, res) {
  if (req.method === 'OPTIONS') return res.status(204).end();

  if (req.method === 'GET') {
    const rec = globalThis.__encomdb_tunnel;
    if (!rec.url) {
      return res.status(200).json({ url: null, updated_at: null, note: 'no tunnel registered yet' });
    }
    return res.status(200).json(rec);
  }

  if (req.method === 'POST') {
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

    globalThis.__encomdb_tunnel = { url, updated_at: new Date().toISOString() };
    return res.status(200).json({ ok: true, ...globalThis.__encomdb_tunnel });
  }

  return res.status(405).json({ error: 'method not allowed' });
}
