package main

import (
	"html/template"
	"net/http"
)

type pageData struct {
	Title   string
	Message string
	Answer  string
	Q       *pendingQuestion
	Form    bool
	Success bool
}

// Plain HTML forms, no JavaScript: works behind ngrok's interstitial, and
// html/template escapes everything the agent or user supplied.
var pageTmpl = template.Must(template.New("page").Funcs(template.FuncMap{"source": sourceLabel}).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>Momentum · {{.Title}}</title>
<style>
  :root { color-scheme: light dark; --bg:#f4f4f6; --card:#fff; --text:#18181b; --muted:#71717a; --accent:#7c3aed; --line:#e4e4e7; --ok:#059669; }
  @media (prefers-color-scheme: dark) { :root { --bg:#09090b; --card:#18181b; --text:#fafafa; --muted:#a1a1aa; --line:#27272a; --accent:#8b5cf6; --ok:#10b981; } }
  * { box-sizing: border-box; }
  body { margin:0; min-height:100vh; display:flex; align-items:center; justify-content:center; padding:20px; background:var(--bg); color:var(--text); font:16px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
  main { width:100%; max-width:460px; background:var(--card); border:1px solid var(--line); border-radius:18px; padding:28px; }
  .src { color:var(--muted); font-size:13px; margin:0 0 6px; }
  h1 { font-size:20px; margin:0 0 18px; }
  .q { white-space:pre-wrap; word-break:break-word; background:var(--bg); border-radius:12px; padding:16px; margin:0 0 20px; }
  form { display:flex; flex-direction:column; gap:10px; margin:0; }
  button { font:inherit; font-weight:600; border:0; border-radius:12px; padding:14px; cursor:pointer; background:var(--accent); color:#fff; }
  .or { text-align:center; color:var(--muted); font-size:13px; margin:8px 0 2px; }
  input { font:inherit; padding:13px; border-radius:12px; border:1px solid var(--line); background:var(--card); color:var(--text); }
  .secondary { background:transparent; color:var(--accent); border:1px solid var(--accent); }
  .msg { color:var(--muted); margin:0 0 16px; }
  .answer { font-weight:600; background:var(--bg); border-radius:12px; padding:14px; margin:0 0 16px; word-break:break-word; }
  .ok { color:var(--ok); }
</style>
</head>
<body>
<main>
  {{with .Q}}{{with source .}}<p class="src">{{.}}</p>{{end}}{{end}}
  <h1{{if .Success}} class="ok"{{end}}>{{if .Success}}✅ {{end}}{{.Title}}</h1>
  {{if .Form}}
    <p class="q">{{.Q.Question}}</p>
    {{with .Message}}<p class="msg">{{.}}</p>{{end}}
    <form method="post">
      {{range .Q.Options}}<button type="submit" name="answer" value="{{.}}">{{.}}</button>{{end}}
    </form>
    <p class="or">or type your own answer</p>
    <!-- Separate form so pressing Enter here can never submit the first option. -->
    <form method="post">
      <input type="text" name="custom" placeholder="Your answer…" autocomplete="off" maxlength="2000" required>
      <button type="submit" class="secondary">Send answer</button>
    </form>
  {{else}}
    {{with .Q}}<p class="q">{{.Question}}</p>{{end}}
    {{with .Answer}}<p class="answer">You answered: {{.}}</p>{{end}}
    <p class="msg">{{.Message}}</p>
  {{end}}
</main>
</body>
</html>`))

func renderPage(w http.ResponseWriter, status int, d pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	w.WriteHeader(status)
	pageTmpl.Execute(w, d)
}
