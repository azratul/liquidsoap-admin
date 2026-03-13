package main

import (
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"radio-web/internal/config"
	"radio-web/internal/handler"
	"radio-web/internal/library"
	"radio-web/internal/liquidsoap"
	mw "radio-web/internal/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	setupLogger(cfg.LogLevel)

	// Templates: base + shared partials, then an isolated clone per page
	funcs := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"pathEsc": func(p string) string {
			parts := strings.Split(p, "/")
			for i, part := range parts {
				parts[i] = url.PathEscape(part)
			}
			return strings.Join(parts, "/")
		},
	}
	baseTmpl, err := template.New("").Funcs(funcs).ParseFiles("web/templates/base.html")
	if err != nil {
		slog.Error("templates base", "err", err)
		os.Exit(1)
	}
	baseTmpl, err = baseTmpl.ParseGlob("web/templates/partials/*.html")
	if err != nil {
		slog.Error("templates partials", "err", err)
		os.Exit(1)
	}

	mustPage := func(file string) *template.Template {
		t, err := baseTmpl.Clone()
		if err != nil {
			slog.Error("clone template", "file", file, "err", err)
			os.Exit(1)
		}
		t, err = t.ParseFiles(file)
		if err != nil {
			slog.Error("parse template", "file", file, "err", err)
			os.Exit(1)
		}
		return t
	}

	lsClient := liquidsoap.NewClient(cfg.LiquidsoaplAddr)
	lastfm := library.NewLastFMClient(cfg.LastFMKey, cfg.LastFMURL)
	browser := library.NewBrowser(cfg.MusicRoot)

	statusH := handler.NewStatusHandler(lsClient, lastfm, browser, cfg.QueueName, mustPage("web/templates/index.html"))
	queueH := handler.NewQueueHandler(lsClient, cfg.MusicRoot, cfg.QueueName, mustPage("web/templates/queue.html"))
	libH := handler.NewLibraryHandler(lsClient, browser, mustPage("web/templates/library.html"), mustPage("web/templates/search.html"))

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	if cfg.AuthEnabled() {
		slog.Info("basic auth enabled", "user", cfg.AuthUser)
		r.Use(mw.BasicAuth(cfg.AuthUser, cfg.AuthPass))
	}

	// static
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// pages
	r.Get("/", statusH.Dashboard)
	r.Get("/library", libH.Root)
	r.Get("/library/*", libH.Browse)
	r.Get("/search", libH.Search)
	r.Get("/queue", queueH.View)

	// actions
	r.Post("/queue/add", queueH.Add)
	r.Delete("/queue/{rid}", queueH.Remove)
	r.Post("/queue/flush", queueH.Flush)
	r.Post("/skip", queueH.Skip)

	// partials HTMX
	r.Get("/partials/now-playing", statusH.NowPlayingPartial)
	r.Get("/partials/queue", queueH.QueuePartial)
	r.Get("/partials/badge", statusH.BadgePartial)

	// API JSON
	r.Get("/api/status", statusH.StatusJSON)

	slog.Info("radio-admin started", "addr", cfg.HTTPAddr(), "music_root", cfg.MusicRoot)
	if err := http.ListenAndServe(cfg.HTTPAddr(), r); err != nil {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}

func setupLogger(level string) {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: l})))
}
