package handler

import (
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"radio-web/internal/library"
	"radio-web/internal/liquidsoap"
)

type LibraryHandler struct {
	ls         *liquidsoap.Client
	browser    *library.Browser
	browseTmpl *template.Template
	searchTmpl *template.Template
}

func NewLibraryHandler(ls *liquidsoap.Client, browser *library.Browser, browseTmpl, searchTmpl *template.Template) *LibraryHandler {
	return &LibraryHandler{
		ls:         ls,
		browser:    browser,
		browseTmpl: browseTmpl,
		searchTmpl: searchTmpl,
	}
}

func (h *LibraryHandler) Root(w http.ResponseWriter, r *http.Request) {
	h.browse(w, r, "")
}

func (h *LibraryHandler) Browse(w http.ResponseWriter, r *http.Request) {
	subPath := chi.URLParam(r, "*")
	if decoded, err := url.PathUnescape(subPath); err == nil {
		subPath = decoded
	}
	h.browse(w, r, subPath)
}

func (h *LibraryHandler) browse(w http.ResponseWriter, r *http.Request, subPath string) {
	listing, err := h.browser.List(subPath)
	if err != nil {
		slog.Warn("library browse", "path", subPath, "err", err)
		http.Error(w, "directory not found", http.StatusNotFound)
		return
	}

	crumbs := library.Breadcrumbs(listing.RelPath)
	np, _ := h.ls.OnAir()
	uptime, _ := h.ls.Uptime()

	data := map[string]any{
		"Listing":     listing,
		"Breadcrumbs": crumbs,
		"NowPlaying":  np,
		"Uptime":      uptime,
	}

	if r.Header.Get("HX-Request") == "true" {
		if err := h.browseTmpl.ExecuteTemplate(w, "library-content.html", data); err != nil {
			slog.Error("template library-content", "err", err)
		}
		return
	}

	if err := h.browseTmpl.ExecuteTemplate(w, "library.html", data); err != nil {
		slog.Error("template library", "err", err)
	}
}

func (h *LibraryHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	if q == "" {
		http.Redirect(w, r, "/library", http.StatusSeeOther)
		return
	}

	results, err := h.browser.Search(q)
	if err != nil {
		slog.Error("library search", "q", q, "err", err)
		http.Error(w, "search error", http.StatusInternalServerError)
		return
	}

	np, _ := h.ls.OnAir()
	uptime, _ := h.ls.Uptime()

	data := map[string]any{
		"Query":      q,
		"Results":    results,
		"NowPlaying": np,
		"Uptime":     uptime,
	}

	if r.Header.Get("HX-Request") == "true" {
		if err := h.searchTmpl.ExecuteTemplate(w, "search-results.html", data); err != nil {
			slog.Error("template search-results", "err", err)
		}
		return
	}

	if err := h.searchTmpl.ExecuteTemplate(w, "search.html", data); err != nil {
		slog.Error("template search", "err", err)
	}
}
