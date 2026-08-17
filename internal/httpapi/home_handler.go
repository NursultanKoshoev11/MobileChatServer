package httpapi

import "net/http"

const koomHomeHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <meta name="description" content="Koom is a mobile platform for communities to communicate, coordinate and make decisions together.">
  <title>Koom - Community Communication</title>
  <style>
    :root{color-scheme:light;--bg:#f5f7fb;--card:#fff;--text:#162033;--muted:#687386;--primary:#5b5ce2;--line:#e5e9f0}
    *{box-sizing:border-box}body{margin:0;font-family:Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:linear-gradient(180deg,#eef1ff 0,#f8f9fc 45%,#fff 100%);color:var(--text)}
    main{max-width:920px;margin:0 auto;padding:72px 24px 56px}.hero{text-align:center;padding:28px 0 38px}.logo{width:76px;height:76px;margin:0 auto 18px;border-radius:24px;display:grid;place-items:center;background:var(--primary);color:#fff;font-weight:900;font-size:36px;box-shadow:0 16px 40px rgba(91,92,226,.28)}
    h1{font-size:clamp(42px,8vw,72px);line-height:1;margin:0 0 16px;letter-spacing:-.04em}.lead{max-width:680px;margin:0 auto;color:var(--muted);font-size:20px;line-height:1.6}
    .grid{display:grid;grid-template-columns:repeat(3,1fr);gap:16px;margin:28px 0}.card{background:rgba(255,255,255,.9);border:1px solid var(--line);border-radius:22px;padding:24px;box-shadow:0 10px 30px rgba(22,32,51,.06)}.card h2{font-size:18px;margin:0 0 9px}.card p{margin:0;color:var(--muted);line-height:1.55}
    .links{display:flex;justify-content:center;gap:12px;flex-wrap:wrap;margin-top:34px}.links a{display:inline-flex;align-items:center;justify-content:center;padding:12px 18px;border:1px solid var(--line);border-radius:14px;background:#fff;color:var(--text);font-weight:700;text-decoration:none}.links a:hover{border-color:#b9bdf8}
    footer{text-align:center;color:var(--muted);font-size:13px;margin-top:48px}@media(max-width:720px){main{padding-top:42px}.grid{grid-template-columns:1fr}.lead{font-size:17px}}
  </style>
</head>
<body>
  <main>
    <section class="hero">
      <div class="logo" aria-label="Koom">K</div>
      <h1>Koom</h1>
      <p class="lead">A mobile community communication platform that helps people connect, share information, coordinate and make decisions together.</p>
    </section>
    <section class="grid" aria-label="Koom features">
      <article class="card"><h2>Community communication</h2><p>Private and public groups for structured communication between community members.</p></article>
      <article class="card"><h2>Requests and collaboration</h2><p>Create community requests, discuss them and work together on shared priorities.</p></article>
      <article class="card"><h2>Mobile-first</h2><p>Koom is designed for secure, convenient communication on modern mobile devices.</p></article>
    </section>
    <nav class="links" aria-label="Legal and safety links">
      <a href="/privacy">Privacy Policy</a>
      <a href="/child-safety">Child Safety Standards</a>
      <a href="/api/health">Service Status</a>
    </nav>
    <footer>(c) 2026 Koom. Community communication platform.</footer>
  </main>
</body>
</html>`

func (s *Server) homePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src 'self' data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(koomHomeHTML))
}
