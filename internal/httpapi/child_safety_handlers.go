package httpapi

import "net/http"

func (s *Server) childSafetyStandards(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(childSafetyStandardsHTML))
}

const childSafetyStandardsHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Koom Child Safety Standards</title>
  <style>
    :root{color-scheme:light;--bg:#f6f8fb;--card:#ffffff;--text:#172033;--muted:#5f6b7a;--line:#d9e1ec;--brand:#0b66d8;}
    *{box-sizing:border-box}
    body{margin:0;background:var(--bg);color:var(--text);font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Arial,sans-serif;line-height:1.65}
    main{max-width:980px;margin:0 auto;padding:32px 18px 56px}
    .card{background:var(--card);border:1px solid var(--line);border-radius:18px;padding:30px;box-shadow:0 8px 30px rgba(17,24,39,.06)}
    h1{font-size:34px;line-height:1.2;margin:0 0 8px}
    h2{font-size:22px;margin:34px 0 10px;border-top:1px solid var(--line);padding-top:22px}
    p,li{font-size:16px} ul{padding-left:22px} .muted{color:var(--muted)} a{color:var(--brand)} .notice{background:#eef6ff;border:1px solid #cfe4ff;border-radius:14px;padding:14px 16px;margin:18px 0}
  </style>
</head>
<body>
<main>
  <article class="card">
    <h1>Koom Child Safety Standards</h1>
    <p class="muted">Last updated: July 9, 2026</p>
    <p>Koom is committed to protecting children and providing a safe communication environment for communities. These standards describe our rules and enforcement approach for child safety.</p>

    <div class="notice">
      <strong>Zero tolerance:</strong> Koom strictly prohibits child sexual abuse material, child sexual exploitation, grooming, solicitation of minors, trafficking, coercion, or any behavior that exploits or endangers children.
    </div>

    <h2>1. Prohibited content and behavior</h2>
    <p>Users must not create, upload, share, request, promote, link to, or distribute any content or activity that involves the sexual exploitation or abuse of children. Users must not use Koom to contact, pressure, manipulate, exploit, or endanger minors.</p>

    <h2>2. Reporting inside the app</h2>
    <p>Users can report safety concerns, harmful content, suspicious accounts, or abusive behavior from within the app where reporting tools are available. Reports are reviewed and may result in content removal, account restrictions, or other safety actions.</p>

    <h2>3. Review and enforcement</h2>
    <p>When we identify or receive a report about potential child safety violations, we may take actions including:</p>
    <ul>
      <li>removing violating content;</li>
      <li>restricting or banning accounts involved in violations;</li>
      <li>preserving relevant information when legally required or necessary for safety;</li>
      <li>blocking repeat abuse and preventing re-upload or re-distribution where technically possible;</li>
      <li>escalating serious cases to appropriate legal authorities or child safety organizations when required by law.</li>
    </ul>

    <h2>4. Cooperation with authorities</h2>
    <p>Koom aims to comply with applicable child safety laws and lawful requests from competent authorities. Where required by law or appropriate to protect children, we may report suspected child safety violations to relevant regional or national authorities.</p>

    <h2>5. Account and community responsibility</h2>
    <p>Community owners, administrators, and users are responsible for following these standards. Communities that allow or encourage child safety violations may be restricted or removed.</p>

    <h2>6. Contact for child safety issues</h2>
    <p>For questions or reports related to child safety standards, contact us at <a href="mailto:nursultankosoev2@gmail.com">nursultankosoev2@gmail.com</a>.</p>

    <h2>7. Updates to these standards</h2>
    <p>We may update these standards as our app, moderation process, legal requirements, and safety practices evolve. The latest version will remain available on this page.</p>
  </article>
</main>
</body>
</html>`