package httpapi

import "net/http"

func setSiteSecurityHeaders(w http.ResponseWriter, allowInlineStyles bool) {
	styleSource := "'self'"
	if allowInlineStyles {
		styleSource = "'unsafe-inline'"
	}

	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src "+styleSource+"; img-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; script-src 'none'")
	w.Header().Set("Strict-Transport-Security", "max-age=31536000")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
}
