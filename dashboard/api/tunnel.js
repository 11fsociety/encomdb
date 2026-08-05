// EncomPortal — tunnel registry endpoint.
//
// GET  /api/tunnel                    → { url, updated_at }
// POST /api/tunnel  { url }           → stores the current phone tunnel URL
//                                        requires Authorization: Bearer <ENCOMDB_TUNNEL_REGISTRY_TOKEN>
//
// Uses Vercel KV (free tier) as the store. If KV isn't configured yet the
// endpoint falls back to a warning payload so the dashboard can still render.

import { kv } from '@vercel/kv';

const KV_KEY = 'encomdb:tunnel';

export default async function handler(req, res) {
  // CORS preflight (headers set at platform level via vercel.json)
  if (req.method === 'OPTIONS') {
    return res.status(204).end();
  }

  if (req.method === 'GET') {
    try {
      const record = await kv.get(KV_KEY);
      if (!record) {
        return res.status(200).json({ url: null, updated_at: null, note: 'no tunnel registered yet' });
      }
      return res.status(200).json(record);
    } catch (err) {
      return res.status(200).json({
        url: null,
        updated_at: null,
        error: 'kv-unavailable',
        detail: String(err?.message || err),
      });
    }
  }

  if (req.method === 'POST') {
    const auth = req.headers.authorization || '';
    const expected = process.env.ENCOMDB_TUNNEL_REGISTRY_TOKEN;
    if (!expected) {
      return res.status(500).json({ error: 'server not configured: ENCOMDB_TUNNEL_REGISTRY_TOKEN unset' });
    }
    if (!auth.toLowerCase().startsWith('bearer ') || auth.slice(7).trim() !== expected) {
      return res.status(401).json({ error: 'invalid token' });
    }

    let body = req.body;
    if (typeof body === 'string') {
      try { body = JSON.parse(body); } catch { body = null; }
    }
    const url = body && typeof body.url === 'string' ? body.url.trim() : '';
    if (!url) {
      return res.status(400).json({ error: 'url required' });
    }
    if (!/^https?:\/\//i.test(url)) {
      return res.status(400).json({ error: 'url must start with http:// or https://' });
    }

    const record = { url, updated_at: new Date().toISOString() };
    try {
      await kv.set(KV_KEY, record);
    } catch (err) {
      return res.status(500).json({ error: 'kv-write-failed', detail: String(err?.message || err) });
    }
    return res.status(200).json({ ok: true, ...record });
  }

  return res.status(405).json({ error: 'method not allowed' });
}
