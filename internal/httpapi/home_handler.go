package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed home.ru.html home.kg.html account_deletion.html site-assets/*
var homeSiteFiles embed.FS

func (s *Server) homePage(w http.ResponseWriter, r *http.Request) {
	page := "home.ru.html"
	if r.URL.Path == "/kg" {
		page = "home.kg.html"
	}

	content, err := homeSiteFiles.ReadFile(page)
	if err != nil {
		http.Error(w, "page unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; img-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; script-src 'none'")
	setSiteSecurityHeaders(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) homeAssets(w http.ResponseWriter, r *http.Request) {
	assets, err := fs.Sub(homeSiteFiles, "site-assets")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.StripPrefix("/assets/", http.FileServer(http.FS(assets))).ServeHTTP(w, r)
}

func (s *Server) accountDeletionPage(w http.ResponseWriter, r *http.Request) {
	content, err := homeSiteFiles.ReadFile("account_deletion.html")
	if err != nil {
		http.Error(w, "page unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, must-revalidate")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
	setSiteSecurityHeaders(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}
