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
      .backdrop{position:fixed;inset:0;background:rgba(0,0,0,.55);display:none;align-items:center;justify-content:center;padding:16px}
      .modal{width:min(520px,94vw);background:#fff;border:1px solid #ddd;border-radius:12px;box-shadow:0 10px 30px rgba(0,0,0,.25);display:flex;flex-direction:column;max-height:86vh}
      .modalHeader{padding:14px 16px;border-bottom:1px solid #eee;font-weight:800;display:flex;justify-content:space-between;gap:10px}
      .modalBody{padding:12px 16px;overflow:auto}
      .modalFooter{padding:12px 16px;border-top:1px solid #eee;display:flex;justify-content:flex-end;gap:8px}
      .modalFooter button{width:auto;margin-top:0}
      .ratesTitle{font-size:26px;font-weight:900;text-align:center;margin:10px 0 8px 0}
      .ratesTable{width:100%;border-collapse:collapse;border-radius:8px;overflow:hidden}
      .ratesTable th{background:#eaf3ff;color:#2b6cb0;font-size:14px;letter-spacing:.08em;text-transform:uppercase;padding:10px;border-bottom:1px solid #e6eef8}
      .ratesTable td{padding:10px;border-bottom:1px solid #f0f3f7;text-align:center}
      .ratesTable td.rate{color:#19b45b;font-weight:900}
      .ratesClose{width:100%;margin-top:0;background:#2f93d8;border:0;color:#fff;padding:14px;border-radius:8px;font-size:18px;font-weight:800}
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

      <div class="row">
        <button type="button" class="secondary" id="insertCoinBtn">Insert Coin</button>
        <button type="button" class="secondary" id="ratesBtn">Rates</button>
      </div>

      <form method="post" action="/login">
        <input type="hidden" name="redir" value="{{.Redir}}">
        <label>Voucher Code</label>
        <input id="code" name="code" autocomplete="one-time-code" placeholder="Enter code" required>
        <button type="submit">Connect</button>
      </form>
    </div>

    <div class="backdrop" id="coinBackdrop">
      <div class="modal">
        <div class="modalHeader">
          <span>Insert Coin</span>
          <span id="coinCountdown" class="muted">60s</span>
        </div>
        <div class="modalBody">
          <div class="kv">
            <div class="k">Inserted</div>
            <div class="v" id="coinAmount">₱0</div>
          </div>
          <div class="kv">
            <div class="k">Time To Add</div>
            <div class="v" id="coinMinutes">0 min</div>
          </div>
          <div class="muted" id="coinHint" style="margin-top:10px">Waiting for coin pulses...</div>
          <div class="error" id="coinErr" style="display:none"></div>
          <div class="ok" id="coinOk" style="display:none"></div>
        </div>
        <div class="modalFooter">
          <button type="button" class="secondary" id="coinCancelBtn">Cancel</button>
          <button type="button" id="coinDoneBtn">Done</button>
        </div>
      </div>
    </div>

    <div class="backdrop" id="ratesBackdrop">
      <div class="modal">
        <div class="modalHeader">
          <span class="muted"></span>
          <span class="muted"></span>
        </div>
        <div class="modalBody">
          <div class="ratesTitle">WiFi Rates</div>
          <table class="ratesTable">
            <thead>
              <tr>
                <th>Rate</th>
                <th>Time</th>
                <th>Speed</th>
                <th>Type</th>
              </tr>
            </thead>
            <tbody id="ratesBody">
              <tr><td colspan="4" class="muted" style="text-align:center;padding:14px">Loading...</td></tr>
            </tbody>
          </table>
        </div>
        <div class="modalFooter">
          <button type="button" class="ratesClose" id="ratesCloseBtn">Close</button>
        </div>
      </div>
    </div>

    <script>
      (() => {
        const statusEl = document.getElementById("status");
        const timerEl = document.getElementById("timer");
        const insertCoinBtn = document.getElementById("insertCoinBtn");
        const ratesBtn = document.getElementById("ratesBtn");

        const coinBackdrop = document.getElementById("coinBackdrop");
        const coinCountdown = document.getElementById("coinCountdown");
        const coinAmount = document.getElementById("coinAmount");
        const coinMinutes = document.getElementById("coinMinutes");
        const coinHint = document.getElementById("coinHint");
        const coinErr = document.getElementById("coinErr");
        const coinOk = document.getElementById("coinOk");
        const coinCancelBtn = document.getElementById("coinCancelBtn");
        const coinDoneBtn = document.getElementById("coinDoneBtn");

        const ratesBackdrop = document.getElementById("ratesBackdrop");
        const ratesCloseBtn = document.getElementById("ratesCloseBtn");
        const ratesBody = document.getElementById("ratesBody");

        let secondsLeft = 0;
        let countdownId = null;
        let coinToken = "";
        let coinPollId = null;

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

        function openCoinModal() {
          coinBackdrop.style.display = "flex";
          coinErr.style.display = "none";
          coinOk.style.display = "none";
          coinErr.textContent = "";
          coinOk.textContent = "";
          coinAmount.textContent = "₱0";
          coinMinutes.textContent = "0 min";
          coinCountdown.textContent = "60s";
          coinHint.textContent = "Starting...";
        }

        function closeCoinModal() {
          coinBackdrop.style.display = "none";
          if (coinPollId) {
            clearInterval(coinPollId);
            coinPollId = null;
          }
        }

        async function coinStart() {
          openCoinModal();
          try {
            const res = await fetch("/api/v1/coin/start", { method: "POST" });
            if (res.status === 409) {
              const d = await res.json().catch(() => null);
              coinErr.style.display = "";
              coinErr.textContent = "Coin slot is busy. Try again in " + ((d && d.seconds_left) || 0) + "s";
              coinHint.textContent = "";
              return;
            }
            if (!res.ok) throw new Error("start failed");
            const d = await res.json();
            coinToken = d.token || "";
            coinHint.textContent = "Insert coin now. Window: " + (d.window_seconds || 60) + "s";
            await coinPoll();
            coinPollId = setInterval(coinPoll, 500);
          } catch (e) {
            coinErr.style.display = "";
            coinErr.textContent = "Coin is not available on this device.";
            coinHint.textContent = "";
          }
        }

        async function coinPoll() {
          if (!coinToken) return;
          try {
            const res = await fetch("/api/v1/coin/status?token=" + encodeURIComponent(coinToken), { cache: "no-store" });
            if (!res.ok) throw new Error("bad");
            const d = await res.json();
            coinCountdown.textContent = String(d.seconds_left || 0) + "s";
            coinAmount.textContent = "₱" + String(d.amount || 0);
            coinMinutes.textContent = String(d.minutes || 0) + " min";
            coinHint.textContent = "Pulses: " + String(d.pulses || 0);
            if ((d.seconds_left || 0) <= 0) {
              if ((d.minutes || 0) > 0) {
                await coinCommit();
              } else {
                await coinCancel();
              }
            }
          } catch {}
        }

        async function coinCancel() {
          if (!coinToken) {
            closeCoinModal();
            return;
          }
          try {
            await fetch("/api/v1/coin/cancel?token=" + encodeURIComponent(coinToken), { method: "POST" });
          } catch {}
          coinToken = "";
          closeCoinModal();
        }

        async function coinCommit() {
          if (!coinToken) return;
          coinDoneBtn.disabled = true;
          try {
            const res = await fetch("/api/v1/coin/commit", { method: "POST", headers: {"Content-Type":"application/json"}, body: JSON.stringify({ token: coinToken }) });
            if (!res.ok) throw new Error("commit failed");
            const d = await res.json();
            coinOk.style.display = "";
            coinOk.textContent = "Added " + String(d.minutes || 0) + " minutes.";
            coinErr.style.display = "none";
            coinToken = "";
            closeCoinModal();
            pollStatus();
          } catch (e) {
            coinErr.style.display = "";
            coinErr.textContent = "Failed to apply time.";
          } finally {
            coinDoneBtn.disabled = false;
          }
        }

        async function openRates() {
          ratesBackdrop.style.display = "flex";
          ratesBody.innerHTML = '<tr><td colspan="4" class="muted" style="text-align:center;padding:14px">Loading...</td></tr>';
          try {
            const res = await fetch("/api/v1/rates", { cache: "no-store" });
            if (!res.ok) throw new Error("bad");
            const d = await res.json();
            const items = (d && d.items) ? d.items : [];
            if (!items.length) {
              ratesBody.innerHTML = '<tr><td colspan="4" class="muted" style="text-align:center;padding:14px">No rates configured.</td></tr>';
              return;
            }
            function fmtTime(mins) {
              const m = Number(mins || 0);
              if (!isFinite(m) || m <= 0) return "-";
              if (m < 60) return m + " Mins";
              if (m % 1440 === 0) return (m / 1440) + " Days";
              if (m % 60 === 0) return (m / 60) + " Hrs";
              const h = Math.floor(m / 60);
              const mm = m % 60;
              return h + " Hrs " + mm + " Mins";
            }
            function fmtSpeed(r) {
              const up = Number(r.up_mbps || 0);
              const down = Number(r.down_mbps || 0);
              if (up > 0 && down > 0) return up + "M/" + down + "M";
              if (up > 0) return up + "M/-";
              if (down > 0) return "-/" + down + "M";
              return "5M/5M";
            }
            function fmtType(r) {
              return r.pause ? "Pausable" : "Regular";
            }
            ratesBody.innerHTML = "";
            items.forEach((r) => {
              const tr = document.createElement("tr");
              const tdRate = document.createElement("td");
              tdRate.className = "rate";
              tdRate.textContent = "₱" + String(r.price || 0);
              const tdTime = document.createElement("td");
              tdTime.textContent = fmtTime(r.minutes);
              const tdSpeed = document.createElement("td");
              tdSpeed.textContent = fmtSpeed(r);
              const tdType = document.createElement("td");
              tdType.textContent = fmtType(r);
              tr.appendChild(tdRate);
              tr.appendChild(tdTime);
              tr.appendChild(tdSpeed);
              tr.appendChild(tdType);
              ratesBody.appendChild(tr);
            });
          } catch {
            ratesBody.innerHTML = '<tr><td colspan="4" class="muted" style="text-align:center;padding:14px">Failed to load rates.</td></tr>';
          }
        }

        function closeRates() {
          ratesBackdrop.style.display = "none";
        }

        insertCoinBtn.addEventListener("click", () => { coinStart(); });
        coinCancelBtn.addEventListener("click", () => { coinCancel(); });
        coinDoneBtn.addEventListener("click", () => { coinCommit(); });

        ratesBtn.addEventListener("click", () => { openRates(); });
        ratesCloseBtn.addEventListener("click", () => { closeRates(); });
        ratesBackdrop.addEventListener("click", (e) => { if (e.target === ratesBackdrop) closeRates(); });

        pollStatus();
        setInterval(pollStatus, 5000);
      })();
    </script>
  </body>
</html>`
