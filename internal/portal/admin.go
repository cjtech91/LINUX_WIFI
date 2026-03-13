package portal

const adminHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{{.Title}} • Admin</title>
    <style>
      :root { --bg:#0f172a; --panel:#1e293b; --muted:#94a3b8; --text:#e2e8f0; --accent:#10b981; --warn:#f59e0b; --danger:#ef4444; }
      *{box-sizing:border-box}
      body{margin:0;height:100vh;background:var(--bg);color:var(--text);font-family:system-ui,-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif}
      .layout{display:flex;height:100vh;overflow:hidden}
      .overlay{position:fixed;inset:0;background:rgba(0,0,0,.55);opacity:0;pointer-events:none;transition:opacity .15s ease}
      body.sidebar-open .overlay{opacity:1;pointer-events:auto}

      .sidebar{width:240px;background:#0b1222;border-right:1px solid #0b152b;padding:14px;display:flex;flex-direction:column;gap:10px;transition:transform .15s ease, width .15s ease}
      .brand{display:flex;align-items:center;gap:10px;font-weight:800}
      .brand .badge{font-size:11px;color:var(--muted);border:1px solid #193657;border-radius:999px;padding:2px 8px}
      .sectionTitle{margin-top:6px;font-size:11px;color:var(--muted);text-transform:uppercase;letter-spacing:.1em}
      .nav{display:flex;flex-direction:column;gap:6px}
      .nav a{display:flex;align-items:center;gap:10px;padding:9px 10px;border-radius:10px;color:var(--muted);text-decoration:none;transition:background .12s ease,color .12s ease}
      .nav a .ico{width:18px;opacity:.9}
      .nav a.active,.nav a:hover{background:#121e35;color:var(--text)}
      .spacer{flex:1}
      .logout{margin-top:8px}
      .logout a{display:flex;align-items:center;justify-content:center;padding:10px 12px;border-radius:10px;background:#2a3446;border:1px solid #35435c;color:var(--text);text-decoration:none}
      .logout a:hover{background:#313d52}

      body.sidebar-collapsed .sidebar{width:72px}
      body.sidebar-collapsed .nav a span.txt{display:none}
      body.sidebar-collapsed .sectionTitle{display:none}
      body.sidebar-collapsed .brand .txt{display:none}
      body.sidebar-collapsed .brand .badge{display:none}

      .main{flex:1;display:flex;flex-direction:column;min-width:0}
      .topbar{display:flex;align-items:center;gap:10px;padding:12px 16px;border-bottom:1px solid #0b152b}
      .topbar .title{font-weight:800}
      .topbar .pill{margin-left:auto;display:flex;align-items:center;gap:8px;color:var(--muted);font-size:12px}
      .topbar .dot{width:10px;height:10px;border-radius:999px;background:var(--accent)}
      .iconBtn{border:1px solid #193657;background:#13223a;color:var(--text);border-radius:10px;padding:8px 10px;cursor:pointer}
      .iconBtn:hover{background:#162846}
      .grid{display:grid;gap:12px;padding:16px;grid-template-columns:repeat(12,1fr)}
      .card{background:var(--panel);border:1px solid #0b152b;border-radius:12px;padding:14px}
      .span-3{grid-column:span 3}
      .span-4{grid-column:span 4}
      .span-6{grid-column:span 6}
      .span-12{grid-column:span 12}
      .title{font-size:12px;color:var(--muted);text-transform:uppercase;letter-spacing:.08em}
      .value{font-size:28px;font-weight:800;margin-top:6px}
      .row{display:flex;gap:8px;margin-top:10px}
      .btn{padding:10px 12px;border:1px solid #193657;border-radius:8px;background:#13223a;color:var(--text);cursor:pointer}
      .btn:hover{background:#162846}
      .table{width:100%;border-collapse:collapse;margin-top:8px}
      .table th,.table td{padding:8px;border-bottom:1px solid #0b152b;font-size:14px;color:var(--muted)}
      .muted{color:var(--muted)}

      @media (max-width:960px){
        body.sidebar-collapsed .sidebar{width:240px}
        .sidebar{position:fixed;z-index:20;top:0;bottom:0;left:0;transform:translateX(-102%)}
        body.sidebar-open .sidebar{transform:translateX(0)}
        .grid{grid-template-columns:repeat(6,1fr)}
        .span-6{grid-column:span 6}
        .span-4{grid-column:span 6}
        .span-3{grid-column:span 6}
      }
    </style>
  </head>
  <body>
    <div class="overlay" id="overlay"></div>
    <div class="layout">
      <aside class="sidebar" id="sidebar">
        <div class="brand">
          <span class="ico">▦</span>
          <span class="txt">{{.Title}}</span>
          <span class="badge">Admin</span>
        </div>

        <div class="sectionTitle">Admin</div>
        <nav class="nav" id="navAdmin">
          <a data-page="dashboard" href="/admin?page=dashboard"><span class="ico">⌂</span><span class="txt">Dashboard</span></a>
        </nav>

        <div class="sectionTitle">Pages</div>
        <nav class="nav" id="navPages">
          <a data-page="interfaces" href="/admin?page=interfaces"><span class="ico">⇄</span><span class="txt">Interfaces</span></a>
          <a data-page="pppoe" href="/admin?page=pppoe"><span class="ico">⛓</span><span class="txt">PPPOE</span></a>
          <a data-page="vouchers" href="/admin?page=vouchers"><span class="ico">⟡</span><span class="txt">Vouchers</span></a>
          <a data-page="network" href="/admin?page=network"><span class="ico">⛭</span><span class="txt">Network</span></a>
          <a data-page="qos" href="/admin?page=qos"><span class="ico">≋</span><span class="txt">QoS</span></a>
          <a data-page="subvendo" href="/admin?page=subvendo"><span class="ico">☷</span><span class="txt">Sub Vendo</span></a>
          <a data-page="portal" href="/admin?page=portal"><span class="ico">⌁</span><span class="txt">Portal</span></a>
          <a data-page="logs" href="/admin?page=logs"><span class="ico">≣</span><span class="txt">Logs</span></a>
          <a data-page="devices" href="/admin?page=devices"><span class="ico">▣</span><span class="txt">Devices</span></a>
          <a data-page="chat" href="/admin?page=chat"><span class="ico">✉</span><span class="txt">Chat</span></a>
          <a data-page="hotspot-sales" href="/admin?page=hotspot-sales"><span class="ico">₱</span><span class="txt">Hotspot Sales</span></a>
          <a data-page="pppoe-sales" href="/admin?page=pppoe-sales"><span class="ico">₱</span><span class="txt">PPPOE Sales</span></a>
          <a data-page="license" href="/admin?page=license"><span class="ico">◈</span><span class="txt">License</span></a>
          <a data-page="settings" href="/admin?page=settings"><span class="ico">⚙</span><span class="txt">Settings</span></a>
        </nav>

        <div class="spacer"></div>

        <div class="logout">
          <a href="/admin?logout=1">Logout</a>
        </div>
      </aside>
      <main class="main">
        <div class="topbar">
          <button class="iconBtn" id="menuBtn" type="button">☰</button>
          <button class="iconBtn" id="collapseBtn" type="button">⇤</button>
          <div class="title" id="pageTitle">Dashboard</div>
          <div class="pill"><span class="dot"></span><span>System Online</span></div>
        </div>
        <section class="grid" id="dashboardSection">
        <div class="card span-3">
          <div class="title">Active Sessions</div>
          <div class="value" id="activeSessions">-</div>
        </div>
        <div class="card span-3">
          <div class="title">Vouchers</div>
          <div class="value" id="vouchers">-</div>
        </div>
        <div class="card span-3">
          <div class="title">Gateway</div>
          <div class="value" id="gateway">-</div>
        </div>
        <div class="card span-3">
          <div class="title">Time (UTC)</div>
          <div class="value" id="timeNow">-</div>
        </div>

        <div class="card span-6">
          <div class="row">
            <button class="btn" onclick="createVoucher(60)">Create Voucher 1h</button>
            <button class="btn" onclick="createVoucher(180)">Create Voucher 3h</button>
            <button class="btn" onclick="createVoucher(1440)">Create Voucher 1d</button>
          </div>
          <div id="voucherOut" class="row" style="margin-top:12px;color:var(--accent)"></div>
        </div>

        <div class="card span-6">
          <div class="title">Recent Activity</div>
          <table class="table">
            <thead><tr><th>Event</th><th>Time</th></tr></thead>
            <tbody id="activity"><tr><td>Loaded dashboard</td><td id="loadTime">-</td></tr></tbody>
          </table>
        </div>
        </section>

        <section class="grid" id="pageSection" style="display:none">
          <div class="card span-12">
            <div class="title" id="pageSectionTitle">Page</div>
            <div class="muted" id="pageSectionHint" style="margin-top:10px"></div>
            <div style="margin-top:12px; overflow:auto">
              <table class="table" id="pageTable" style="display:none">
                <thead id="pageTableHead"></thead>
                <tbody id="pageTableBody"></tbody>
              </table>
            </div>
          </div>
        </section>
      </main>
    </div>
    <script>
      async function fetchSummary() {
        try {
          const res = await fetch('/api/admin/summary', {cache:'no-store'});
          if (!res.ok) throw new Error('bad');
          const data = await res.json();
          document.getElementById('activeSessions').textContent = data.active_sessions ?? '-';
          document.getElementById('vouchers').textContent = data.vouchers ?? '-';
          document.getElementById('gateway').textContent = data.gateway_ip ?? '-';
          document.getElementById('timeNow').textContent = data.time_utc ?? '-';
          if (document.getElementById('boardModel')) {
            document.getElementById('boardModel').textContent = data.board_model ?? '-';
            if (data.gpio) {
              const d = data.gpio.disabled ? 'disabled' : 'enabled';
              document.getElementById('gpioInfo').textContent =
                'GPIO ' + d +
                ' • coin ' + data.gpio.coin_pin + ' (' + data.gpio.coin_edge + ')' +
                ' • bill ' + data.gpio.bill_pin + ' (' + data.gpio.bill_edge + ')' +
                ' • relay ' + data.gpio.relay_pin + ' (' + data.gpio.relay_active + ')';
            }
          }
        } catch (e) {
          document.getElementById('activeSessions').textContent = '-';
        }
      }
      async function createVoucher(mins) {
        try {
          const res = await fetch('/api/v1/vouchers', {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify({minutes: mins})});
          if (!res.ok) throw new Error('create failed');
          const v = await res.json();
          document.getElementById('voucherOut').textContent = 'New voucher: ' + v.code + ' ('+v.minutes+' mins)';
          addActivity('Voucher created: '+v.code);
          fetchSummary();
        } catch (e) {
          document.getElementById('voucherOut').textContent = 'Failed to create voucher';
        }
      }
      function addActivity(text) {
        const tb = document.getElementById('activity');
        const tr = document.createElement('tr');
        const td1 = document.createElement('td'); td1.textContent = text;
        const td2 = document.createElement('td'); td2.textContent = new Date().toISOString();
        tr.appendChild(td1); tr.appendChild(td2); tb.prepend(tr);
      }
      document.getElementById('loadTime').textContent = new Date().toISOString();

      function setTable(headCols, rows) {
        const head = document.getElementById('pageTableHead');
        const body = document.getElementById('pageTableBody');
        head.innerHTML = '';
        body.innerHTML = '';

        const trh = document.createElement('tr');
        headCols.forEach(c => {
          const th = document.createElement('th');
          th.textContent = c;
          trh.appendChild(th);
        });
        head.appendChild(trh);

        rows.forEach(r => {
          const tr = document.createElement('tr');
          r.forEach(cell => {
            const td = document.createElement('td');
            td.textContent = cell;
            tr.appendChild(td);
          });
          body.appendChild(tr);
        });

        document.getElementById('pageTable').style.display = '';
      }

      async function loadInterfaces() {
        document.getElementById('pageTable').style.display = 'none';
        document.getElementById('pageSectionHint').textContent = 'Loading interfaces...';
        try {
          const res = await fetch('/api/admin/interfaces', {cache:'no-store'});
          if (!res.ok) throw new Error('bad');
          const data = await res.json();
          const rows = (data.interfaces || []).map(i => ([
            i.name || '',
            (i.flags || []).join(' '),
            String(i.mtu ?? ''),
            i.hardware_addr || '',
            (i.addrs || []).join(', '),
          ]));
          document.getElementById('pageSectionHint').textContent =
            'Default: ' + (data.default_interface || '-') + ' via ' + (data.default_gateway || '-');
          setTable(['Name', 'Flags', 'MTU', 'MAC', 'Addresses'], rows);
        } catch (e) {
          document.getElementById('pageSectionHint').textContent = 'Failed to load interfaces';
        }
      }

      async function loadVouchers() {
        document.getElementById('pageTable').style.display = 'none';
        document.getElementById('pageSectionHint').textContent = 'Loading vouchers...';
        try {
          const res = await fetch('/api/admin/vouchers?limit=200', {cache:'no-store'});
          if (!res.ok) throw new Error('bad');
          const data = await res.json();
          const items = data.items || [];
          const rows = items.map(v => ([
            v.code || '',
            String(v.minutes ?? ''),
            v.created_at_utc || '',
            v.used_at_utc || '',
            v.used_by_ip || '',
            v.used_by_mac || '',
          ]));
          document.getElementById('pageSectionHint').textContent = 'Latest vouchers (most recent first)';
          setTable(['Code', 'Minutes', 'Created (UTC)', 'Used (UTC)', 'Used IP', 'Used MAC'], rows);
        } catch (e) {
          document.getElementById('pageSectionHint').textContent = 'Failed to load vouchers';
        }
      }

      async function loadLogs() {
        document.getElementById('pageTable').style.display = 'none';
        document.getElementById('pageSectionHint').textContent = 'Loading logs...';
        try {
          const res = await fetch('/api/admin/logs?limit=200', {cache:'no-store'});
          if (!res.ok) throw new Error('bad');
          const data = await res.json();
          const items = data.items || [];
          const rows = items.map(e => ([
            e.time_utc || '',
            e.type || '',
            e.message || '',
            e.client_ip || '',
          ]));
          document.getElementById('pageSectionHint').textContent = 'Latest events (DB-derived)';
          setTable(['Time (UTC)', 'Type', 'Message', 'Client IP'], rows);
        } catch (e) {
          document.getElementById('pageSectionHint').textContent = 'Failed to load logs';
        }
      }

      function setPageFromQuery() {
        const params = new URLSearchParams(window.location.search);
        const page = (params.get('page') || '{{.Page}}' || 'dashboard').toLowerCase();
        const title = page.replaceAll('-', ' ').replaceAll('_', ' ');
        const nice = title.charAt(0).toUpperCase() + title.slice(1);
        document.getElementById('pageTitle').textContent = nice === '' ? 'Dashboard' : nice;

        const all = document.querySelectorAll('.nav a[data-page]');
        all.forEach(a => a.classList.remove('active'));
        const active = document.querySelector('.nav a[data-page="'+page+'"]');
        if (active) active.classList.add('active');

        if (page === 'dashboard') {
          document.getElementById('dashboardSection').style.display = '';
          document.getElementById('pageSection').style.display = 'none';
        } else {
          document.getElementById('dashboardSection').style.display = 'none';
          document.getElementById('pageSection').style.display = '';
          document.getElementById('pageSectionTitle').textContent = nice;
          document.getElementById('pageSectionHint').textContent = '';
          if (page === 'interfaces') {
            loadInterfaces();
          } else if (page === 'vouchers') {
            loadVouchers();
          } else if (page === 'logs') {
            loadLogs();
          } else {
            document.getElementById('pageSectionHint').textContent = 'Coming soon';
          }
        }
      }

      function openSidebar() { document.body.classList.add('sidebar-open'); }
      function closeSidebar() { document.body.classList.remove('sidebar-open'); }
      function toggleCollapse() {
        const collapsed = document.body.classList.toggle('sidebar-collapsed');
        try { localStorage.setItem('admin_sidebar_collapsed', collapsed ? '1' : '0'); } catch {}
      }
      function initCollapseFromStorage() {
        try {
          const v = localStorage.getItem('admin_sidebar_collapsed');
          if (v === '1') document.body.classList.add('sidebar-collapsed');
        } catch {}
      }

      document.getElementById('menuBtn').addEventListener('click', () => {
        if (document.body.classList.contains('sidebar-open')) closeSidebar(); else openSidebar();
      });
      document.getElementById('collapseBtn').addEventListener('click', () => toggleCollapse());
      document.getElementById('overlay').addEventListener('click', () => closeSidebar());
      document.querySelectorAll('.sidebar a').forEach(a => a.addEventListener('click', () => closeSidebar()));

      initCollapseFromStorage();
      setPageFromQuery();
      fetchSummary();
      setInterval(fetchSummary, 5000);
    </script>
  </body>
  </html>`
