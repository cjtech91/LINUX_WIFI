package portal

const adminHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{{.Title}} • Admin</title>
    <style>
      :root { --bg:#0f172a; --panel:#1e293b; --muted:#94a3b8; --text:#e2e8f0; --accent:#10b981; --warn:#f59e0b; --danger:#ef4444; }
      *{box-sizing:border-box} body{margin:0;display:flex;height:100vh;background:var(--bg);color:var(--text);font-family:system-ui,-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif}
      .sidebar{width:220px;background:#0b1222;border-right:1px solid #0b152b;padding:16px;display:flex;flex-direction:column;gap:8px}
      .brand{font-weight:700;margin-bottom:12px}
      .nav a{display:block;padding:10px 12px;border-radius:8px;color:var(--muted);text-decoration:none}
      .nav a.active,.nav a:hover{background:#121e35;color:var(--text)}
      .main{flex:1;display:flex;flex-direction:column}
      .topbar{padding:12px 16px;border-bottom:1px solid #0b152b}
      .grid{display:grid;gap:12px;padding:16px;grid-template-columns:repeat(12,1fr)}
      .card{background:var(--panel);border:1px solid #0b152b;border-radius:12px;padding:14px}
      .span-3{grid-column:span 3}
      .span-4{grid-column:span 4}
      .span-6{grid-column:span 6}
      .title{font-size:12px;color:var(--muted);text-transform:uppercase;letter-spacing:.08em}
      .value{font-size:28px;font-weight:800;margin-top:6px}
      .row{display:flex;gap:8px;margin-top:10px}
      .btn{padding:10px 12px;border:1px solid #193657;border-radius:8px;background:#13223a;color:var(--text);cursor:pointer}
      .btn:hover{background:#162846}
      .table{width:100%;border-collapse:collapse;margin-top:8px}
      .table th,.table td{padding:8px;border-bottom:1px solid #0b152b;font-size:14px;color:var(--muted)}
      @media (max-width:960px){.sidebar{display:none}.grid{grid-template-columns:repeat(6,1fr)}.span-6{grid-column:span 6}.span-4{grid-column:span 6}.span-3{grid-column:span 6}}
    </style>
  </head>
  <body>
    <aside class="sidebar">
      <div class="brand">{{.Title}} Admin</div>
      <nav class="nav">
        <a class="active" href="/admin">Dashboard</a>
        <a href="#" onclick="alert('Coming soon')">Interfaces</a>
        <a href="#" onclick="alert('Coming soon')">Vouchers</a>
        <a href="#" onclick="alert('Coming soon')">Logs</a>
        <a href="#" onclick="alert('Coming soon')">Settings</a>
      </nav>
      <div style="margin-top:auto">
        <a class="nav" style="padding:0" href="/">← Back to Portal</a>
      </div>
    </aside>
    <main class="main">
      <div class="topbar">
        <strong>Dashboard</strong>
      </div>
      <section class="grid">
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
    </main>
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
      fetchSummary(); setInterval(fetchSummary, 5000);
    </script>
  </body>
  </html>`

