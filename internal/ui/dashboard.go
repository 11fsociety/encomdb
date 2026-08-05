package ui

// DashboardHTML is a single-file SPA served at /dashboard.
// It uses the browser's fetch API and the currently logged-in PocketBase admin token
// (the admin needs to be logged into /_/ in another tab, or paste a token).
//
// Kept intentionally as one embed-friendly string so there's no build step on Termux.
const DashboardHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width,initial-scale=1" />
  <title>EncomDB — Dashboard</title>
  <script src="https://cdn.tailwindcss.com"></script>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
    pre { white-space: pre-wrap; word-break: break-all; }
  </style>
</head>
<body class="bg-gray-950 text-gray-100 min-h-screen">
  <nav class="border-b border-gray-800 px-4 py-3 flex items-center justify-between">
    <div class="flex items-center gap-3">
      <span class="font-bold text-lg">EncomDB</span>
      <span class="text-xs text-gray-400">v0.1.0</span>
    </div>
    <div class="flex items-center gap-3 text-sm">
      <a href="/_/" class="text-gray-400 hover:text-white">PB Admin</a>
      <a href="/api/health" class="text-gray-400 hover:text-white">Health</a>
    </div>
  </nav>

  <main class="max-w-5xl mx-auto p-6">
    <div id="auth-panel" class="mb-6 p-4 rounded-lg bg-gray-900 border border-gray-800 hidden">
      <h2 class="font-semibold mb-2">Authenticate</h2>
      <p class="text-sm text-gray-400 mb-3">Sign in with your PocketBase superuser account to manage databases.</p>
      <div class="flex gap-2 flex-wrap">
        <input id="admin-email" type="email" placeholder="admin@example.com" class="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm min-w-0" />
        <input id="admin-password" type="password" placeholder="password" class="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm min-w-0" />
        <button id="btn-login" class="bg-white text-black px-4 py-2 rounded text-sm font-medium hover:bg-gray-200">Sign in</button>
      </div>
      <p id="auth-error" class="text-red-400 text-sm mt-2 hidden"></p>
    </div>

    <div id="app" class="hidden">
      <div class="flex items-center justify-between mb-6">
        <div>
          <h1 class="text-2xl font-bold">Databases</h1>
          <p class="text-gray-400 text-sm">Manage your SQL databases and get connection strings.</p>
        </div>
        <div class="flex items-center gap-2 flex-wrap">
          <span id="tunnel-badge" class="hidden text-xs px-2 py-1 rounded bg-green-900/40 text-green-300 border border-green-800/50 font-mono"></span>
          <span id="admin-email-badge" class="text-xs text-gray-400"></span>
          <button id="btn-logout" class="text-xs text-gray-400 hover:text-white">Sign out</button>
          <button id="btn-new" class="bg-white text-black px-4 py-2 rounded text-sm font-medium hover:bg-gray-200">
            + New database
          </button>
        </div>
      </div>

      <div id="empty" class="hidden text-center py-16 text-gray-500">
        No databases yet. Click "New database" to create your first one.
      </div>

      <div id="dbs" class="grid gap-3"></div>
    </div>
  </main>

  <!-- Create DB modal -->
  <div id="modal-new" class="fixed inset-0 bg-black/70 hidden items-center justify-center p-4 z-40">
    <div class="bg-gray-900 border border-gray-800 rounded-lg max-w-md w-full p-6">
      <h3 class="font-bold text-lg mb-4">Create database</h3>
      <label class="block text-sm text-gray-400 mb-1">Name</label>
      <input id="new-name" type="text" placeholder="my_project"
        class="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm mb-3" />
      <label class="block text-sm text-gray-400 mb-1">Description (optional)</label>
      <input id="new-desc" type="text" placeholder="What is this database for?"
        class="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm mb-4" />
      <p id="new-error" class="text-red-400 text-sm mb-3 hidden"></p>
      <div class="flex gap-2 justify-end">
        <button id="btn-cancel-new" class="text-gray-400 hover:text-white px-4 py-2 text-sm">Cancel</button>
        <button id="btn-create-new" class="bg-white text-black px-4 py-2 rounded text-sm font-medium hover:bg-gray-200">Create</button>
      </div>
    </div>
  </div>

  <!-- Connection info modal -->
  <div id="modal-info" class="fixed inset-0 bg-black/70 hidden items-center justify-center p-4 z-40">
    <div class="bg-gray-900 border border-gray-800 rounded-lg max-w-2xl w-full p-6">
      <div class="flex items-center justify-between mb-4">
        <h3 class="font-bold text-lg">Connection info</h3>
        <button id="btn-close-info" class="text-gray-400 hover:text-white text-2xl leading-none">&times;</button>
      </div>
      <div class="space-y-3 text-sm">
        <div>
          <div class="text-gray-400 mb-1">Endpoint</div>
          <div class="flex gap-2">
            <input id="info-endpoint" readonly class="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 font-mono text-xs" />
            <button data-copy="info-endpoint" class="bg-gray-800 hover:bg-gray-700 rounded px-3 py-2 text-xs">Copy</button>
          </div>
        </div>
        <div>
          <div class="text-gray-400 mb-1">API key</div>
          <div class="flex gap-2">
            <input id="info-apikey" readonly class="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 font-mono text-xs" />
            <button data-copy="info-apikey" class="bg-gray-800 hover:bg-gray-700 rounded px-3 py-2 text-xs">Copy</button>
          </div>
        </div>
        <div>
          <div class="text-gray-400 mb-1">Sample curl</div>
          <pre id="info-curl" class="bg-gray-800 border border-gray-700 rounded p-3 font-mono text-xs overflow-x-auto"></pre>
          <button data-copy="info-curl" data-copy-mode="text" class="mt-1 bg-gray-800 hover:bg-gray-700 rounded px-3 py-1 text-xs">Copy curl</button>
        </div>
      </div>
    </div>
  </div>

  <script>
  (function(){
    const API = "/api/encom";
    const $ = (id) => document.getElementById(id);
    const el = {
      auth: $("auth-panel"), app: $("app"),
      dbs: $("dbs"), empty: $("empty"),
      modalNew: $("modal-new"), modalInfo: $("modal-info"),
      badge: $("admin-email-badge"),
    };

    function tokenKey() { return "pb_admin_token"; }
    function emailKey() { return "pb_admin_email"; }
    function getToken() { return localStorage.getItem(tokenKey()) || ""; }
    function setToken(t, email) {
      if (t) { localStorage.setItem(tokenKey(), t); }
      else { localStorage.removeItem(tokenKey()); localStorage.removeItem(emailKey()); }
      if (email) localStorage.setItem(emailKey(), email);
    }

    function show(el) { el.classList.remove("hidden"); if (el.classList.contains("modal-open")) return; }
    function hide(el) { el.classList.add("hidden"); }
    function showFlex(el) { el.classList.remove("hidden"); el.classList.add("flex"); }
    function hideFlex(el) { el.classList.add("hidden"); el.classList.remove("flex"); }

    async function api(path, opts={}) {
      const headers = Object.assign({"Content-Type": "application/json"}, opts.headers || {});
      const tok = getToken();
      if (tok) headers["Authorization"] = tok;
      const res = await fetch(path, Object.assign({}, opts, {headers}));
      const text = await res.text();
      let body = null; try { body = text ? JSON.parse(text) : null; } catch { body = text; }
      if (!res.ok) {
        const msg = (body && body.message) || (typeof body === "string" ? body : "request failed");
        throw new Error(msg + " (HTTP " + res.status + ")");
      }
      return body;
    }

    async function login(email, password) {
      const res = await fetch("/api/collections/_superusers/auth-with-password", {
        method: "POST",
        headers: {"Content-Type":"application/json"},
        body: JSON.stringify({identity: email, password: password}),
      });
      if (!res.ok) throw new Error("invalid credentials (HTTP " + res.status + ")");
      const body = await res.json();
      setToken(body.token, email);
    }

    function bytes(n){
      if (n < 1024) return n + " B";
      if (n < 1024*1024) return (n/1024).toFixed(1) + " KB";
      return (n/1024/1024).toFixed(2) + " MB";
    }

    async function refresh() {
      try {
        const list = await api(API + "/dbs");
        if (!Array.isArray(list) || list.length === 0) {
          hide(el.dbs); show(el.empty);
          el.dbs.innerHTML = "";
          return;
        }
        show(el.dbs); hide(el.empty);
        el.dbs.innerHTML = list.map(function(db){
          const badge = db.status === "ready"
            ? '<span class="text-green-400 text-xs">ready</span>'
            : db.status === "pending"
              ? '<span class="text-yellow-400 text-xs">provisioning…</span>'
              : '<span class="text-red-400 text-xs">error</span>';
          return '' +
          '<div class="bg-gray-900 border border-gray-800 rounded-lg p-4 flex items-center justify-between gap-4">' +
            '<div class="min-w-0">' +
              '<div class="flex items-center gap-2 mb-1">' +
                '<span class="font-mono font-semibold">' + esc(db.name) + '</span>' +
                badge +
              '</div>' +
              '<div class="text-sm text-gray-400 truncate">' + esc(db.description || '') + '</div>' +
              '<div class="text-xs text-gray-500 mt-1">' + bytes(db.size_bytes || 0) + '</div>' +
            '</div>' +
            '<div class="flex gap-2 shrink-0">' +
              '<button data-connect="' + esc(db.name) + '" class="bg-gray-800 hover:bg-gray-700 rounded px-3 py-1 text-xs">Connect</button>' +
              '<button data-delete="' + esc(db.name) + '" class="bg-red-900/40 hover:bg-red-900/70 rounded px-3 py-1 text-xs">Delete</button>' +
            '</div>' +
          '</div>';
        }).join("");
      } catch (e) {
        if (String(e.message).includes("401")) {
          setToken("", "");
          renderAuthState();
        } else {
          alert(e.message);
        }
      }
    }

    function esc(s){ return String(s).replace(/[&<>"']/g, function(c){
      return {"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[c];
    }); }

    async function createDB(){
      const name = $("new-name").value.trim();
      const desc = $("new-desc").value.trim();
      const err = $("new-error");
      err.classList.add("hidden"); err.textContent = "";
      try {
        await api(API + "/dbs", { method: "POST", body: JSON.stringify({name, description: desc}) });
        hideFlex(el.modalNew);
        $("new-name").value = ""; $("new-desc").value = "";
        await refresh();
      } catch (e) {
        err.classList.remove("hidden"); err.textContent = e.message;
      }
    }

    async function deleteDB(name){
      if (!confirm("Delete database \"" + name + "\"? This is permanent.")) return;
      await api(API + "/dbs/" + encodeURIComponent(name), { method: "DELETE" });
      await refresh();
    }

    async function showConnect(name){
      const info = await api(API + "/dbs/" + encodeURIComponent(name));
      $("info-endpoint").value = info.sql_endpoint;
      $("info-apikey").value = info.api_key;
      $("info-curl").textContent = info.curl_example;
      showFlex(el.modalInfo);
    }

    function copyText(text){
      navigator.clipboard.writeText(text).then(function(){}, function(){});
    }

    async function refreshTunnel(){
      const tb = document.getElementById("tunnel-badge");
      if (!tb) return;
      try {
        const info = await api(API + "/tunnel");
        if (info && info.tunnel_url) {
          tb.textContent = "Public: " + info.tunnel_url.replace(/^https?:\/\//, "");
          tb.classList.remove("hidden");
          tb.title = info.tunnel_url;
        } else {
          tb.textContent = "LAN only";
          tb.classList.remove("hidden");
        }
      } catch (_) {}
    }

    function renderAuthState(){
      const tok = getToken();
      if (tok) {
        hide(el.auth); show(el.app);
        el.badge.textContent = localStorage.getItem(emailKey()) || "";
        refresh();
        refreshTunnel();
      } else {
        show(el.auth); hide(el.app);
      }
    }

    document.addEventListener("click", function(evt){
      const t = evt.target;
      if (t.id === "btn-new") { showFlex(el.modalNew); return; }
      if (t.id === "btn-cancel-new") { hideFlex(el.modalNew); return; }
      if (t.id === "btn-create-new") { createDB(); return; }
      if (t.id === "btn-close-info") { hideFlex(el.modalInfo); return; }
      if (t.id === "btn-logout") { setToken("", ""); renderAuthState(); return; }
      if (t.id === "btn-login") {
        const email = $("admin-email").value.trim();
        const password = $("admin-password").value;
        const err = $("auth-error");
        err.classList.add("hidden"); err.textContent = "";
        login(email, password).then(function(){ renderAuthState(); },
          function(e){ err.classList.remove("hidden"); err.textContent = e.message; });
        return;
      }
      const del = t.getAttribute && t.getAttribute("data-delete");
      if (del) { deleteDB(del); return; }
      const cn = t.getAttribute && t.getAttribute("data-connect");
      if (cn) { showConnect(cn); return; }
      const cp = t.getAttribute && t.getAttribute("data-copy");
      if (cp) {
        const src = document.getElementById(cp);
        const text = t.getAttribute("data-copy-mode") === "text" ? src.textContent : src.value;
        copyText(text);
        t.textContent = "Copied";
        setTimeout(function(){ t.textContent = t.getAttribute("data-copy-mode") === "text" ? "Copy curl" : "Copy"; }, 1200);
        return;
      }
    });

    renderAuthState();
    setInterval(function(){ if (getToken()) { refresh(); refreshTunnel(); } }, 5000);
  })();
  </script>
</body>
</html>`
