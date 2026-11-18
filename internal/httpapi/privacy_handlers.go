package httpapi

import "net/http"

func (s *Server) privacyPolicy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, must-revalidate")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; script-src 'none'")
	setSiteSecurityHeaders(w)
	_, _ = w.Write([]byte(privacyPolicyHTML))
}

const privacyPolicyHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Koom Privacy Policy</title>
  <style>
    :root{color-scheme:light;--bg:#f7f9ff;--card:#ffffff;--text:#10254f;--muted:#617191;--line:#dce6f5;--brand:#0878f9;}
    *{box-sizing:border-box}
    body{margin:0;background:linear-gradient(180deg,#edf5ff 0,#f7f9ff 280px);color:var(--text);font-family:Inter,-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Arial,sans-serif;line-height:1.7}
    .policy-top{height:72px;display:flex;align-items:center;justify-content:space-between;padding:0 max(22px,calc((100% - 980px)/2));border-bottom:1px solid #dce6f5;background:#f7f9ffeb}
    .policy-brand{display:flex;align-items:center;gap:10px;color:var(--text);font-size:20px;font-weight:850;letter-spacing:-.04em;text-decoration:none}
    .policy-brand img{width:36px;height:36px;border-radius:12px}
    .policy-home{color:#506383;font-size:13px;font-weight:700;text-decoration:none}.policy-home:hover{color:var(--brand)}
    main{max-width:980px;margin:0 auto;padding:38px 18px 64px}
    .card{background:var(--card);border:1px solid var(--line);border-radius:22px;padding:clamp(24px,5vw,46px);box-shadow:0 18px 55px rgba(17,45,86,.08)}
    h1{font-size:clamp(30px,5vw,40px);line-height:1.15;letter-spacing:-.045em;margin:0 0 8px}
    h2{font-size:21px;letter-spacing:-.025em;margin:32px 0 10px;border-top:1px solid var(--line);padding-top:22px}
    p,li{font-size:15px} ul{padding-left:22px} li{padding-left:3px;margin:6px 0}.muted{color:var(--muted)} .brand{color:var(--brand);font-weight:700} a{color:var(--brand)}
    @media(max-width:600px){.policy-top{height:64px;padding:0 18px}main{padding:22px 14px 40px}.card{border-radius:17px}p,li{font-size:14px}}
  </style>
</head>
<body>
<header class="policy-top"><a class="policy-brand" href="/"><img src="/assets/koom-icon.png" alt="">KOOM</a><a class="policy-home" href="/">Back to KOOM ↗</a></header>
<main>
  <article class="card">
    <h1>Koom Privacy Policy</h1>
    <p class="muted">Last updated: November 17, 2026</p>
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

    <h2>7. User choices and account deletion</h2>
    <p>You can delete your account from Profile settings in the Koom app or start deletion at <a href="/account-deletion">koom.com.kg/account-deletion</a>. Deletion removes your profile, phone number, active sessions, push tokens, votes, group chat messages, and other account-linked records. Public requests and comments remain with the author identity removed; their text and attached media remain part of the public record. Groups you own are transferred to an existing group admin or member. If you are the only member of a group, invite another member before deleting your account.</p>
    <p>Account deletion from the live service may not immediately remove information from protected backups. Backup copies are removed according to their configured retention schedule. For access or correction requests, or if you cannot access the phone number on your account, contact us at <a href="mailto:nursultankosoev2@gmail.com">nursultankosoev2@gmail.com</a>.</p>

    <h2>8. Children</h2>
    <p>Koom is not intended for children under the minimum age required by applicable law. If we learn that personal information from a child was collected without appropriate consent where consent is required, we will take reasonable steps to delete it.</p>

    <h2>9. International processing</h2>
    <p>Information may be processed and stored on servers or services located outside your country. We take reasonable steps to protect information according to this Privacy Policy wherever it is processed.</p>

    <h2>10. Changes to this policy</h2>
    <p>We may update this Privacy Policy from time to time. When we make changes, we will update the date at the top of this page. Continued use of Koom after an update means the updated policy applies.</p>

    <h2>11. Contact</h2>
    <p>For privacy questions or requests, contact us at <a href="mailto:nursultankosoev2@gmail.com">nursultankosoev2@gmail.com</a>.</p>
  </article>
</main>
<footer style="max-width:980px;margin:0 auto;padding:0 18px 40px;color:#617191;font-size:13px"><a href="/terms">Terms of Use / Условия использования</a> · <a href="/account-deletion">Account deletion</a> · <a href="/child-safety">Child safety</a></footer>
</body>
</html>`
