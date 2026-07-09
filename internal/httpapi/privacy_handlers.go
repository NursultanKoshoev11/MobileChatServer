package httpapi

import "net/http"

func (s *Server) privacyPolicy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(privacyPolicyHTML))
}

const privacyPolicyHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Koom Privacy Policy</title>
  <style>
    :root{color-scheme:light;--bg:#f6f8fb;--card:#ffffff;--text:#172033;--muted:#5f6b7a;--line:#d9e1ec;--brand:#0b66d8;}
    *{box-sizing:border-box}
    body{margin:0;background:var(--bg);color:var(--text);font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Arial,sans-serif;line-height:1.65}
    main{max-width:980px;margin:0 auto;padding:32px 18px 56px}
    .card{background:var(--card);border:1px solid var(--line);border-radius:18px;padding:30px;box-shadow:0 8px 30px rgba(17,24,39,.06)}
    h1{font-size:34px;line-height:1.2;margin:0 0 8px}
    h2{font-size:22px;margin:34px 0 10px;border-top:1px solid var(--line);padding-top:22px}
    p,li{font-size:16px} ul{padding-left:22px} .muted{color:var(--muted)} .brand{color:var(--brand);font-weight:700} a{color:var(--brand)}
  </style>
</head>
<body>
<main>
  <article class="card">
    <h1>Koom Privacy Policy</h1>
    <p class="muted">Last updated: July 9, 2026</p>
    <p>Koom is a community communication application for groups, posts, comments, voting, public requests, media sharing, notifications, and related community features. This Privacy Policy explains what information we collect, how we use it, and what choices users have.</p>

    <h2>1. Information we collect</h2>
    <p>Depending on how you use Koom, we may collect the following categories of information:</p>
    <ul>
      <li><strong>Account and contact information:</strong> phone number used for sign-in and verification, user ID, display name, profile details, group membership, and app role.</li>
      <li><strong>User-generated content:</strong> groups, posts, public requests, comments, votes, messages, files, images, videos, and other media that you choose to create, upload, or share in the app.</li>
      <li><strong>App activity:</strong> actions needed to provide app features, such as creating posts, joining groups, submitting public requests, voting, commenting, and using moderation or administration features.</li>
      <li><strong>Device and technical information:</strong> app version, device type, operating system, IP address, request logs, error logs, security events, diagnostics, and performance information.</li>
      <li><strong>Notification data:</strong> push notification tokens and notification preferences if notifications are enabled on your device.</li>
    </ul>

    <h2>2. How we use information</h2>
    <p>We use information to:</p>
    <ul>
      <li>create, verify, and manage user accounts;</li>
      <li>provide groups, posts, comments, voting, public requests, messages, media sharing, and related app features;</li>
      <li>send notifications, service messages, and important account or security updates;</li>
      <li>prevent abuse, spam, fraud, unauthorized access, and security incidents;</li>
      <li>moderate content and enforce app rules;</li>
      <li>diagnose errors, maintain reliability, improve performance, and develop new features;</li>
      <li>comply with legal, security, and operational requirements.</li>
    </ul>

    <h2>3. Sharing of information</h2>
    <p>We do not sell personal information. We may share or process information with trusted service providers that help us operate the app, including hosting, databases, storage, security, notification delivery, analytics, diagnostics, and support. We may also disclose information when required by law, to protect users, to investigate abuse, or to protect the security and integrity of Koom.</p>

    <h2>4. Public and group content</h2>
    <p>Content submitted to groups, public requests, comments, votes, messages, or similar features may be visible to other users depending on the feature, group settings, roles, and permissions. Do not submit information that you do not want other permitted users to see.</p>

    <h2>5. Data retention</h2>
    <p>We keep information for as long as needed to provide Koom, maintain security, resolve disputes, enforce rules, improve the service, and meet legal or operational requirements. Some technical logs and backups may be retained for a limited period for security, troubleshooting, and recovery purposes. When information is no longer needed, we delete it or de-identify it where reasonably possible.</p>

    <h2>6. Security</h2>
    <p>We use reasonable technical and organizational measures to protect information against unauthorized access, loss, misuse, alteration, or disclosure. No online service can be guaranteed to be completely secure, but we work to protect user accounts, app data, and service infrastructure.</p>

    <h2>7. User choices and requests</h2>
    <p>You may contact us to request access, correction, or deletion of your personal information where applicable. Some information may need to be retained when required for legal, security, fraud prevention, backup, dispute resolution, or operational reasons.</p>

    <h2>8. Children</h2>
    <p>Koom is not intended for children under the minimum age required by applicable law. If we learn that personal information from a child was collected without appropriate consent where consent is required, we will take reasonable steps to delete it.</p>

    <h2>9. International processing</h2>
    <p>Information may be processed and stored on servers or services located outside your country. We take reasonable steps to protect information according to this Privacy Policy wherever it is processed.</p>

    <h2>10. Changes to this policy</h2>
    <p>We may update this Privacy Policy from time to time. When we make changes, we will update the date at the top of this page. Continued use of Koom after an update means the updated policy applies.</p>

    <h2>11. Contact</h2>
    <p>For privacy questions or requests, contact us at <a href="mailto:orozobekovanazgul66@gmail.com">orozobekovanazgul66@gmail.com</a>.</p>
  </article>
</main>
</body>
</html>`