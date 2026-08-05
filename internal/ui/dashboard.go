package ui

// DashboardHTML is a single-file SPA served at /dashboard.
// It uses the browser's fetch API and the currently logged-in admin token
// (the admin needs to be logged into /_/ in another tab, or paste a token).
//
// Kept intentionally as one embed-friendly string so there's no build step on Termux.
const DashboardHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width,initial-scale=1" />
  <title>EncomPortal</title>
  <script src="https://cdn.tailwindcss.com"></script>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
    pre { white-space: pre-wrap; word-break: break-all; }
    .tab-active { border-bottom-color: white; color: white; }
    .tab-inactive { border-bottom-color: transparent; color: #9ca3af; }
  </style>
</head>
<body class="bg-gray-950 text-gray-100 min-h-screen">
  <nav class="border-b border-gray-800 px-4 py-3 flex items-center justify-between">
    <div class="flex items-center gap-3">
      <span class="font-bold text-lg">EncomPortal</span>
      <span class="text-xs text-gray-400">v0.2.0</span>
      <span id="tunnel-badge" class="hidden text-xs px-2 py-1 rounded bg-green-900/40 text-green-300 border border-green-800/50 font-mono"></span>
    </div>
    <div class="flex items-center gap-3 text-sm">
      <a href="/_/" class="text-gray-400 hover:text-white">Admin</a>
      <a href="/api/health" class="text-gray-400 hover:text-white">Health</a>
    </div>
  </nav>

  <main class="max-w-5xl mx-auto p-6">
    <div id="auth-panel" class="mb-6 p-4 rounded-lg bg-gray-900 border border-gray-800 hidden">
      <h2 class="font-semibold mb-2">Authenticate</h2>
      <p class="text-sm text-gray-400 mb-3">Sign in with your admin account to manage the portal.</p>
      <div class="flex gap-2 flex-wrap">
        <input id="admin-email" type="email" placeholder="admin@example.com" class="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm min-w-0" />
        <input id="admin-password" type="password" placeholder="password" class="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm min-w-0" />
        <button id="btn-login" class="bg-white text-black px-4 py-2 rounded text-sm font-medium hover:bg-gray-200">Sign in</button>
      </div>
      <p id="auth-error" class="text-red-400 text-sm mt-2 hidden"></p>
    </div>

    <div id="app" class="hidden">
      <div class="flex items-center justify-between mb-4 flex-wrap gap-2">
        <div class="flex items-center gap-4 border-b border-gray-800 grow">
          <button data-tab="dbs"     class="tab-btn px-3 py-2 text-sm border-b-2 tab-active">RocketDB</button>
          <button data-tab="auth"    class="tab-btn px-3 py-2 text-sm border-b-2 tab-inactive">Auth</button>
          <button data-tab="storage" class="tab-btn px-3 py-2 text-sm border-b-2 tab-inactive">Storage</button>
          <button data-tab="stream"  class="tab-btn px-3 py-2 text-sm border-b-2 tab-inactive">Streaming</button>
          <button data-tab="hosting" class="tab-btn px-3 py-2 text-sm border-b-2 tab-inactive">Hosting</button>
        </div>
        <div class="flex items-center gap-2 flex-wrap">
          <span id="admin-email-badge" class="text-xs text-gray-400"></span>
          <button id="btn-logout" class="text-xs text-gray-400 hover:text-white">Sign out</button>
        </div>
      </div>

      <section id="tab-dbs" class="tab-panel">
        <div class="flex items-center justify-between mb-4">
          <p class="text-gray-400 text-sm">Managed SQL databases running on your phone.</p>
          <button id="btn-new-db" class="bg-white text-black px-4 py-2 rounded text-sm font-medium hover:bg-gray-200">+ New database</button>
        </div>
        <div id="dbs-empty" class="hidden text-center py-16 text-gray-500">No databases yet.</div>
        <div id="dbs" class="grid gap-3"></div>
      </section>

      <section id="tab-auth" class="tab-panel hidden">
        <div class="flex items-center justify-between mb-4">
          <p class="text-gray-400 text-sm">Multi-tenant auth. One app per project, each with its own users and JWT.</p>
          <button id="btn-new-app" class="bg-white text-black px-4 py-2 rounded text-sm font-medium hover:bg-gray-200">+ New app</button>
        </div>
        <div id="apps-empty" class="hidden text-center py-16 text-gray-500">No apps yet.</div>
        <div id="apps" class="grid gap-3"></div>
      </section>

      <section id="tab-storage" class="tab-panel hidden text-center py-16 text-gray-500">Storage (Google Drive) — coming next session.</section>
      <section id="tab-stream"  class="tab-panel hidden text-center py-16 text-gray-500">Streaming — coming after Storage.</section>
      <section id="tab-hosting" class="tab-panel hidden text-center py-16 text-gray-500">Hosting — coming last.</section>
    </div>
  </main>

  <div id="modal-new-db" class="fixed inset-0 bg-black/70 hidden items-center justify-center p-4 z-40">
    <div class="bg-gray-900 border border-gray-800 rounded-lg max-w-md w-full p-6">
      <h3 class="font-bold text-lg mb-4">Create database</h3>
      <label class="block text-sm text-gray-400 mb-1">Name</label>
      <input id="new-db-name" type="text" placeholder="my_project" class="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm mb-3" />
      <label class="block text-sm text-gray-400 mb-1">Description (optional)</label>
      <input id="new-db-desc" type="text" placeholder="What is this database for?" class="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm mb-4" />
      <p id="new-db-error" class="text-red-400 text-sm mb-3 hidden"></p>
      <div class="flex gap-2 justify-end">
        <button data-close="modal-new-db" class="text-gray-400 hover:text-white px-4 py-2 text-sm">Cancel</button>
        <button id="btn-create-db" class="bg-white text-black px-4 py-2 rounded text-sm font-medium hover:bg-gray-200">Create</button>
      </div>
    </div>
  </div>

  <div id="modal-new-app" class="fixed inset-0 bg-black/70 hidden items-center justify-center p-4 z-40">
    <div class="bg-gray-900 border border-gray-800 rounded-lg max-w-md w-full p-6">
      <h3 class="font-bold text-lg mb-4">Create app</h3>
      <label class="block text-sm text-gray-400 mb-1">Name</label>
      <input id="new-app-name" type="text" placeholder="My Product" class="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm mb-3" />
      <label class="block text-sm text-gray-400 mb-1">Slug</label>
      <input id="new-app-slug" type="text" placeholder="my_product" class="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm font-mono mb-3" />
      <label class="block text-sm text-gray-400 mb-1">Description (optional)</label>
      <input id="new-app-desc" type="text" placeholder="What is this app for?" class="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm mb-4" />
      <p id="new-app-error" class="text-red-400 text-sm mb-3 hidden"></p>
      <div class="flex gap-2 justify-end">
        <button data-close="modal-new-app" class="text-gray-400 hover:text-white px-4 py-2 text-sm">Cancel</button>
        <button id="btn-create-app" class="bg-white text-black px-4 py-2 rounded text-sm font-medium hover:bg-gray-200">Create</button>
      </div>
    </div>
  </div>

  <div id="modal-db-info" class="fixed inset-0 bg-black/70 hidden items-center justify-center p-4 z-40">
    <div class="bg-gray-900 border border-gray-800 rounded-lg max-w-2xl w-full p-6">
      <div class="flex items-center justify-between mb-4">
        <h3 class="font-bold text-lg">Connection info</h3>
        <button data-close="modal-db-info" class="text-gray-400 hover:text-white text-2xl leading-none">&times;</button>
      </div>
      <div class="space-y-3 text-sm">
        <div>
          <div class="text-gray-400 mb-1">Endpoint</div>
          <div class="flex gap-2">
            <input id="db-info-endpoint" readonly class="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 font-mono text-xs" />
            <button data-copy-input="db-info-endpoint" class="bg-gray-800 hover:bg-gray-700 rounded px-3 py-2 text-xs">Copy</button>
          </div>
        </div>
        <div>
          <div class="text-gray-400 mb-1">API key</div>
          <div class="flex gap-2">
            <input id="db-info-apikey" readonly class="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 font-mono text-xs" />
            <button data-copy-input="db-info-apikey" class="bg-gray-800 hover:bg-gray-700 rounded px-3 py-2 text-xs">Copy</button>
          </div>
        </div>
        <div>
          <div class="text-gray-400 mb-1">Sample curl</div>
          <pre id="db-info-curl" class="bg-gray-800 border border-gray-700 rounded p-3 font-mono text-xs overflow-x-auto"></pre>
          <button data-copy-text="db-info-curl" class="mt-1 bg-gray-800 hover:bg-gray-700 rounded px-3 py-1 text-xs">Copy curl</button>
        </div>
      </div>
    </div>
  </div>

  <div id="modal-app" class="fixed inset-0 bg-black/70 hidden items-center justify-center p-4 z-40">
    <div class="bg-gray-900 border border-gray-800 rounded-lg max-w-3xl w-full p-6 max-h-[85vh] overflow-y-auto">
      <div class="flex items-center justify-between mb-4">
        <h3 class="font-bold text-lg"><span id="app-detail-name"></span> <span class="text-xs text-gray-500 font-mono" id="app-detail-slug"></span></h3>
        <button data-close="modal-app" class="text-gray-400 hover:text-white text-2xl leading-none">&times;</button>
      </div>
      <div class="grid gap-4 text-sm">
        <div>
          <div class="text-gray-400 mb-1">Publishable key</div>
          <div class="flex gap-2">
            <input id="app-detail-pk" readonly class="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 font-mono text-xs" />
            <button data-copy-input="app-detail-pk" class="bg-gray-800 hover:bg-gray-700 rounded px-3 py-2 text-xs">Copy</button>
          </div>
        </div>
        <div>
          <div class="text-gray-400 mb-1">Secret key (server-side only, shown once at rotate)</div>
          <div class="flex gap-2">
            <input id="app-detail-sk" readonly placeholder="rotate to reveal" class="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 font-mono text-xs" />
            <button id="btn-rotate-secret" class="bg-yellow-900/40 hover:bg-yellow-900/70 border border-yellow-800/50 rounded px-3 py-2 text-xs">Rotate</button>
          </div>
        </div>
        <div>
          <div class="text-gray-400 mb-1">Signup endpoint (public)</div>
          <pre id="app-detail-signup" class="bg-gray-800 border border-gray-700 rounded p-3 font-mono text-xs overflow-x-auto"></pre>
        </div>
        <div>
          <div class="text-gray-400 mb-1">Login endpoint (public)</div>
          <pre id="app-detail-login" class="bg-gray-800 border border-gray-700 rounded p-3 font-mono text-xs overflow-x-auto"></pre>
        </div>
        <div>
          <div class="flex items-center justify-between mb-1">
            <div class="text-gray-400">Users</div>
            <span id="app-detail-user-count" class="text-xs text-gray-500"></span>
          </div>
          <div id="app-detail-users" class="border border-gray-800 rounded divide-y divide-gray-800 overflow-hidden"></div>
        </div>
        <div class="flex justify-end pt-2 border-t border-gray-800">
          <button id="btn-delete-app" class="bg-red-900/40 hover:bg-red-900/70 border border-red-800/50 rounded px-3 py-1 text-xs">Delete app</button>
        </div>
      </div>
    </div>
  </div>

  <script>
  (function(){
    let CURRENT_APP = null;

    const $ = (id) => document.getElementById(id);
    const el = {
      auth: $("auth-panel"), app: $("app"),
      badge: $("admin-email-badge"),
    };

    function tokenKey() { return "pb_admin_token"; }
    function emailKey() { return "pb_admin_email"; }
    function getToken() { return localStorage.getItem(tokenKey()) || ""; }
    function setToken(t, email) {
      if (t) localStorage.setItem(tokenKey(), t);
      else { localStorage.removeItem(tokenKey()); localStorage.removeItem(emailKey()); }
      if (email) localStorage.setItem(emailKey(), email);
    }

    function show(node) { node.classList.remove("hidden"); }
    function hide(node) { node.classList.add("hidden"); }
    function showFlex(node) { node.classList.remove("hidden"); node.classList.add("flex"); }
    function hideFlex(node) { node.classList.add("hidden"); node.classList.remove("flex"); }
    function esc(s){ return String(s == null ? "" : s).replace(/[&<>"']/g, function(c){
      return {"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[c];
    }); }
    function bytes(n){
      if (n < 1024) return n + " B";
      if (n < 1024*1024) return (n/1024).toFixed(1) + " KB";
      return (n/1024/1024).toFixed(2) + " MB";
    }
    function copyText(text){ navigator.clipboard.writeText(text).then(function(){}, function(){}); }

    async function api(path, opts) {
      opts = opts || {};
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
        method: "POST", headers: {"Content-Type":"application/json"},
        body: JSON.stringify({identity: email, password: password}),
      });
      if (!res.ok) throw new Error("invalid credentials (HTTP " + res.status + ")");
      const body = await res.json();
      setToken(body.token, email);
    }

    async function refreshTunnel(){
      const tb = $("tunnel-badge");
      try {
        const info = await api("/api/rocketdb/tunnel");
        if (info && info.tunnel_url) {
          tb.textContent = "Public: " + info.tunnel_url.replace(/^https?:\/\//, "");
          show(tb); tb.title = info.tunnel_url;
        } else {
          tb.textContent = "LAN only"; show(tb);
        }
      } catch (_) {}
    }

    function switchTab(name){
      document.querySelectorAll(".tab-btn").forEach(function(b){
        if (b.getAttribute("data-tab") === name) {
          b.classList.remove("tab-inactive"); b.classList.add("tab-active");
        } else {
          b.classList.remove("tab-active"); b.classList.add("tab-inactive");
        }
      });
      document.querySelectorAll(".tab-panel").forEach(function(p){ hide(p); });
      show($("tab-" + name));
      if (name === "dbs")  refreshDBs();
      if (name === "auth") refreshApps();
    }

    async function refreshDBs() {
      if (!getToken()) return;
      try {
        const list = await api("/api/rocketdb/dbs");
        const grid = $("dbs"); const empty = $("dbs-empty");
        if (!Array.isArray(list) || list.length === 0) {
          hide(grid); show(empty); grid.innerHTML = ""; return;
        }
        show(grid); hide(empty);
        grid.innerHTML = list.map(function(db){
          const badge = db.status === "ready"
            ? '<span class="text-green-400 text-xs">ready</span>'
            : db.status === "pending"
              ? '<span class="text-yellow-400 text-xs">provisioning...</span>'
              : '<span class="text-red-400 text-xs">error</span>';
          return '<div class="bg-gray-900 border border-gray-800 rounded-lg p-4 flex items-center justify-between gap-4">'
            + '<div class="min-w-0">'
              + '<div class="flex items-center gap-2 mb-1"><span class="font-mono font-semibold">' + esc(db.name) + '</span>' + badge + '</div>'
              + '<div class="text-sm text-gray-400 truncate">' + esc(db.description || '') + '</div>'
              + '<div class="text-xs text-gray-500 mt-1">' + bytes(db.size_bytes || 0) + '</div>'
            + '</div>'
            + '<div class="flex gap-2 shrink-0">'
              + '<button data-connect-db="' + esc(db.name) + '" class="bg-gray-800 hover:bg-gray-700 rounded px-3 py-1 text-xs">Connect</button>'
              + '<button data-delete-db="' + esc(db.name) + '" class="bg-red-900/40 hover:bg-red-900/70 rounded px-3 py-1 text-xs">Delete</button>'
            + '</div></div>';
        }).join("");
      } catch (e) {
        if (String(e.message).includes("401")) { setToken("", ""); renderAuthState(); }
      }
    }

    async function createDB(){
      const name = $("new-db-name").value.trim();
      const desc = $("new-db-desc").value.trim();
      const err = $("new-db-error"); hide(err);
      try {
        await api("/api/rocketdb/dbs", { method: "POST", body: JSON.stringify({name, description: desc}) });
        hideFlex($("modal-new-db"));
        $("new-db-name").value = ""; $("new-db-desc").value = "";
        await refreshDBs();
      } catch (e) { show(err); err.textContent = e.message; }
    }

    async function deleteDB(name){
      if (!confirm("Delete database \"" + name + "\"? This is permanent.")) return;
      await api("/api/rocketdb/dbs/" + encodeURIComponent(name), { method: "DELETE" });
      await refreshDBs();
    }

    async function connectDB(name){
      const info = await api("/api/rocketdb/dbs/" + encodeURIComponent(name));
      $("db-info-endpoint").value = info.sql_endpoint;
      $("db-info-apikey").value = info.api_key;
      $("db-info-curl").textContent = info.curl_example;
      showFlex($("modal-db-info"));
    }

    async function refreshApps() {
      if (!getToken()) return;
      try {
        const list = await api("/api/auth/apps");
        const grid = $("apps"); const empty = $("apps-empty");
        if (!Array.isArray(list) || list.length === 0) {
          hide(grid); show(empty); grid.innerHTML = ""; return;
        }
        show(grid); hide(empty);
        grid.innerHTML = list.map(function(a){
          return '<div class="bg-gray-900 border border-gray-800 rounded-lg p-4 flex items-center justify-between gap-4">'
            + '<div class="min-w-0">'
              + '<div class="font-semibold">' + esc(a.name) + '</div>'
              + '<div class="text-xs text-gray-500 font-mono">' + esc(a.slug) + '</div>'
              + '<div class="text-sm text-gray-400 truncate">' + esc(a.description || '') + '</div>'
            + '</div>'
            + '<div class="flex gap-2 shrink-0">'
              + '<button data-open-app="' + esc(a.slug) + '" class="bg-gray-800 hover:bg-gray-700 rounded px-3 py-1 text-xs">Manage</button>'
            + '</div></div>';
        }).join("");
      } catch (e) {
        if (String(e.message).includes("401")) { setToken("", ""); renderAuthState(); }
      }
    }

    async function createApp(){
      const name = $("new-app-name").value.trim();
      const slug = $("new-app-slug").value.trim();
      const desc = $("new-app-desc").value.trim();
      const err = $("new-app-error"); hide(err);
      try {
        const created = await api("/api/auth/apps", { method: "POST", body: JSON.stringify({name, slug, description: desc}) });
        hideFlex($("modal-new-app"));
        $("new-app-name").value = ""; $("new-app-slug").value = ""; $("new-app-desc").value = "";
        await refreshApps();
        openApp(created.slug, created.secret_key);
      } catch (e) { show(err); err.textContent = e.message; }
    }

    async function openApp(slug, revealSecret){
      const detail = await api("/api/auth/apps/" + encodeURIComponent(slug));
      CURRENT_APP = detail;
      $("app-detail-name").textContent = detail.name;
      $("app-detail-slug").textContent = detail.slug;
      $("app-detail-pk").value = detail.publishable_key;
      $("app-detail-sk").value = revealSecret || "";
      const base = location.origin + "/api/auth/apps/" + encodeURIComponent(detail.slug);
      $("app-detail-signup").textContent = "curl -X POST " + base + "/signup \\\n  -H 'Content-Type: application/json' \\\n  -d '{\"email\":\"user@example.com\",\"password\":\"hunter2hunter2\",\"name\":\"User\"}'";
      $("app-detail-login").textContent  = "curl -X POST " + base + "/login  \\\n  -H 'Content-Type: application/json' \\\n  -d '{\"email\":\"user@example.com\",\"password\":\"hunter2hunter2\"}'";
      const users = await api("/api/auth/apps/" + encodeURIComponent(slug) + "/users");
      renderAppUsers(users, slug);
      showFlex($("modal-app"));
    }

    function renderAppUsers(users, slug){
      $("app-detail-user-count").textContent = users.length + " user" + (users.length === 1 ? "" : "s");
      const wrap = $("app-detail-users");
      if (!users.length) { wrap.innerHTML = '<div class="p-4 text-center text-gray-500 text-xs">no users yet</div>'; return; }
      wrap.innerHTML = users.map(function(u){
        return '<div class="p-3 flex items-center justify-between gap-2 text-xs">'
          + '<div class="min-w-0">'
            + '<div class="font-mono">' + esc(u.email) + (u.disabled ? ' <span class="text-red-400">(disabled)</span>' : '') + '</div>'
            + '<div class="text-gray-500">' + esc(u.name || '') + ' - ' + esc(u.id) + '</div>'
          + '</div>'
          + '<div class="flex gap-1 shrink-0">'
            + '<button data-toggle-user="' + esc(u.id) + '" data-user-disabled="' + (u.disabled ? "1" : "0") + '" data-user-slug="' + esc(slug) + '" class="bg-gray-800 hover:bg-gray-700 rounded px-2 py-1">' + (u.disabled ? 'Enable' : 'Disable') + '</button>'
            + '<button data-delete-user="' + esc(u.id) + '" data-user-slug="' + esc(slug) + '" class="bg-red-900/40 hover:bg-red-900/70 rounded px-2 py-1">Delete</button>'
          + '</div></div>';
      }).join("");
    }

    async function rotateSecret(){
      if (!CURRENT_APP) return;
      if (!confirm("Rotate secret key for " + CURRENT_APP.slug + "? Old key stops working immediately.")) return;
      const r = await api("/api/auth/apps/" + encodeURIComponent(CURRENT_APP.slug) + "/rotate-secret", { method: "POST" });
      $("app-detail-sk").value = r.secret_key;
    }

    async function deleteAppConfirm(){
      if (!CURRENT_APP) return;
      if (!confirm("Delete app \"" + CURRENT_APP.slug + "\" and ALL its users? Cannot be undone.")) return;
      await api("/api/auth/apps/" + encodeURIComponent(CURRENT_APP.slug), { method: "DELETE" });
      hideFlex($("modal-app"));
      CURRENT_APP = null;
      await refreshApps();
    }

    async function toggleUser(userId, slug, wasDisabled){
      await api("/api/auth/apps/" + encodeURIComponent(slug) + "/users/" + encodeURIComponent(userId),
        { method: "PATCH", body: JSON.stringify({disabled: !wasDisabled}) });
      await openApp(slug);
    }
    async function deleteUser(userId, slug){
      if (!confirm("Delete user " + userId + "?")) return;
      await api("/api/auth/apps/" + encodeURIComponent(slug) + "/users/" + encodeURIComponent(userId),
        { method: "DELETE" });
      await openApp(slug);
    }

    function renderAuthState(){
      const tok = getToken();
      if (tok) {
        hide(el.auth); show(el.app);
        el.badge.textContent = localStorage.getItem(emailKey()) || "";
        refreshDBs();
        refreshTunnel();
      } else {
        show(el.auth); hide(el.app);
      }
    }

    document.addEventListener("click", function(evt){
      const t = evt.target;
      const id = t.id;
      const attr = function(k){ return t.getAttribute && t.getAttribute(k); };

      const tab = attr("data-tab");
      if (tab) { switchTab(tab); return; }
      const close = attr("data-close");
      if (close) { hideFlex($(close)); return; }

      if (id === "btn-login") {
        const email = $("admin-email").value.trim();
        const password = $("admin-password").value;
        const err = $("auth-error"); hide(err);
        login(email, password).then(function(){ renderAuthState(); },
          function(e){ show(err); err.textContent = e.message; });
        return;
      }
      if (id === "btn-logout") { setToken("", ""); renderAuthState(); return; }

      if (id === "btn-new-db")    { showFlex($("modal-new-db")); return; }
      if (id === "btn-create-db") { createDB(); return; }
      const cnDb = attr("data-connect-db"); if (cnDb) { connectDB(cnDb); return; }
      const delDb = attr("data-delete-db"); if (delDb) { deleteDB(delDb); return; }

      if (id === "btn-new-app")    { showFlex($("modal-new-app")); return; }
      if (id === "btn-create-app") { createApp(); return; }
      const openA = attr("data-open-app"); if (openA) { openApp(openA); return; }
      if (id === "btn-rotate-secret") { rotateSecret(); return; }
      if (id === "btn-delete-app")    { deleteAppConfirm(); return; }
      const togU = attr("data-toggle-user");
      if (togU) { toggleUser(togU, attr("data-user-slug"), attr("data-user-disabled") === "1"); return; }
      const delU = attr("data-delete-user");
      if (delU) { deleteUser(delU, attr("data-user-slug")); return; }

      const cpi = attr("data-copy-input");
      if (cpi) { copyText($(cpi).value); t.textContent = "Copied"; setTimeout(function(){ t.textContent = "Copy"; }, 1200); return; }
      const cpt = attr("data-copy-text");
      if (cpt) { copyText($(cpt).textContent); t.textContent = "Copied"; setTimeout(function(){ t.textContent = "Copy curl"; }, 1200); return; }
    });

    renderAuthState();
    setInterval(function(){
      if (getToken()) {
        const activeTab = document.querySelector(".tab-btn.tab-active");
        const t = activeTab ? activeTab.getAttribute("data-tab") : "dbs";
        if (t === "dbs")  refreshDBs();
        if (t === "auth") refreshApps();
        refreshTunnel();
      }
    }, 5000);
  })();
  </script>
</body>
</html>`
