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
      label { display:block; margin-top: 0.75rem; font-weight: 600; }
      input { width: 100%; padding: 0.6rem; margin-top: 0.35rem; box-sizing: border-box; }
      button { margin-top: 1rem; padding: 0.7rem 0.9rem; width: 100%; }
      .muted { color: #666; font-size: 0.9rem; }
      .error { color: #b00020; margin-top: 0.75rem; }
      .ok { color: #0a7a2f; margin-top: 0.75rem; }
    </style>
  </head>
  <body>
    <div class="card">
      <h2>{{.Title}}</h2>
      <div class="muted">Your IP: {{.ClientIP}}</div>

      {{if .Error}}<div class="error">{{.Error}}</div>{{end}}
      {{if .Ok}}<div class="ok">{{.Ok}}</div>{{end}}

      <form method="post" action="/login">
        <input type="hidden" name="redir" value="{{.Redir}}">
        <label>Voucher Code</label>
        <input name="code" autocomplete="one-time-code" placeholder="Enter code" required>
        <button type="submit">Connect</button>
      </form>
    </div>
  </body>
</html>`

