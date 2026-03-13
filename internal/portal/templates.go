package portal

const portalHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Title}}</title>
    <style>
      body { font-family: system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial, sans-serif; margin: 2rem; }
      .card { max-width: 420px; padding: 1.25rem; border: 1px solid #ddd; border-radius: 10px; }
      .row { display: flex; gap: 0.75rem; margin-top: 0.75rem; }
      .row > * { flex: 1; }
      label { display:block; margin-top: 0.75rem; font-weight: 600; }
      input { width: 100%; padding: 0.6rem; margin-top: 0.35rem; box-sizing: border-box; }
      button { margin-top: 1rem; padding: 0.7rem 0.9rem; width: 100%; }
      button.secondary { background: #f2f2f2; border: 1px solid #ddd; }
      .kv { margin-top: 0.75rem; padding: 0.75rem; border-radius: 10px; background: #fafafa; border: 1px solid #eee; }
      .kv .k { color: #666; font-size: 0.85rem; }
      .kv .v { font-weight: 700; font-size: 1.05rem; margin-top: 0.2rem; }
      .muted { color: #666; font-size: 0.9rem; }
      .error { color: #b00020; margin-top: 0.75rem; }
      .ok { color: #0a7a2f; margin-top: 0.75rem; }
    </style>
  </head>
  <body>
    <div class="card">
      <h2>{{.Title}}</h2>
      <div class="muted">Your IP: <span id="clientIp">{{.ClientIP}}</span></div>

      <div class="row">
        <div class="kv">
          <div class="k">Status</div>
          <div class="v" id="status">Checking...</div>
        </div>
        <div class="kv">
          <div class="k">Time Left</div>
          <div class="v" id="timer">--:--</div>
        </div>
      </div>

      {{if .Error}}<div class="error">{{.Error}}</div>{{end}}
      {{if .Ok}}<div class="ok">{{.Ok}}</div>{{end}}

      <button type="button" class="secondary" id="insertCoinBtn">Insert Coin</button>

      <form method="post" action="/login">
        <input type="hidden" name="redir" value="{{.Redir}}">
        <label>Voucher Code</label>
        <input id="code" name="code" autocomplete="one-time-code" placeholder="Enter code" required>
        <button type="submit">Connect</button>
      </form>
    </div>

    <script>
      (() => {
        const statusEl = document.getElementById("status");
        const timerEl = document.getElementById("timer");
        const codeEl = document.getElementById("code");
        const insertCoinBtn = document.getElementById("insertCoinBtn");
        let secondsLeft = 0;
        let countdownId = null;

        function formatSeconds(total) {
          if (typeof total !== "number" || !isFinite(total) || total < 0) return "--:--";
          const mins = Math.floor(total / 60);
          const secs = total % 60;
          return String(mins).padStart(2, "0") + ":" + String(secs).padStart(2, "0");
        }

        function setCountdown(sec) {
          secondsLeft = Math.max(0, Math.floor(sec || 0));
          timerEl.textContent = formatSeconds(secondsLeft);
          if (countdownId !== null) {
            clearInterval(countdownId);
            countdownId = null;
          }
          if (secondsLeft > 0) {
            countdownId = setInterval(() => {
              if (secondsLeft > 0) secondsLeft -= 1;
              timerEl.textContent = formatSeconds(secondsLeft);
              if (secondsLeft <= 0 && countdownId !== null) {
                clearInterval(countdownId);
                countdownId = null;
              }
            }, 1000);
          }
        }

        async function pollStatus() {
          try {
            const res = await fetch("/api/v1/status", { cache: "no-store" });
            if (!res.ok) throw new Error("bad status");
            const data = await res.json();
            if (data && data.active) {
              statusEl.textContent = "Connected";
              setCountdown(Number(data.seconds_left || 0));
            } else {
              statusEl.textContent = "Not connected";
              setCountdown(0);
            }
          } catch {
            statusEl.textContent = "Unable to check";
            timerEl.textContent = "--:--";
          }
        }

        insertCoinBtn.addEventListener("click", () => {
          statusEl.textContent = "Insert coin, then enter voucher code.";
          codeEl.focus();
        });

        pollStatus();
        setInterval(pollStatus, 5000);
      })();
    </script>
  </body>
</html>`
