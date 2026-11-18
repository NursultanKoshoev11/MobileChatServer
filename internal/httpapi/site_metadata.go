package httpapi

import (
	"net/http"
	"strings"
)

const robotsTxtContent = "User-agent: *\nAllow: /\n\nSitemap: https://koom.com.kg/sitemap.xml\n"

const sitemapXMLContent = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">
  <url>
    <loc>https://koom.com.kg/</loc>
    <xhtml:link rel="alternate" hreflang="ru-KG" href="https://koom.com.kg/" />
    <xhtml:link rel="alternate" hreflang="ky-KG" href="https://koom.com.kg/kg" />
    <xhtml:link rel="alternate" hreflang="x-default" href="https://koom.com.kg/" />
  </url>
  <url>
    <loc>https://koom.com.kg/kg</loc>
    <xhtml:link rel="alternate" hreflang="ru-KG" href="https://koom.com.kg/" />
    <xhtml:link rel="alternate" hreflang="ky-KG" href="https://koom.com.kg/kg" />
    <xhtml:link rel="alternate" hreflang="x-default" href="https://koom.com.kg/" />
  </url>
  <url><loc>https://koom.com.kg/privacy</loc></url>
  <url><loc>https://koom.com.kg/terms</loc></url>
  <url><loc>https://koom.com.kg/child-safety</loc></url>
</urlset>`

func (s *Server) robotsTxt(w http.ResponseWriter, r *http.Request) {
	writeSiteMetadata(w, r, "text/plain; charset=utf-8", robotsTxtContent)
}

func (s *Server) sitemapXML(w http.ResponseWriter, r *http.Request) {
	writeSiteMetadata(w, r, "application/xml; charset=utf-8", sitemapXMLContent)
}

func writeSiteMetadata(w http.ResponseWriter, r *http.Request, contentType, content string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte(strings.TrimSpace(content) + "\n"))
}
