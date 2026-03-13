package handler

import (
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"

	"radio-web/internal/library"
	"radio-web/internal/liquidsoap"
)

type StatusHandler struct {
	ls        *liquidsoap.Client
	lastfm    *library.LastFMClient
	browser   *library.Browser
	queueName string
	tmpl      *template.Template
}

func NewStatusHandler(ls *liquidsoap.Client, lastfm *library.LastFMClient, browser *library.Browser, queueName string, tmpl *template.Template) *StatusHandler {
	return &StatusHandler{
		ls:        ls,
		lastfm:    lastfm,
		browser:   browser,
		queueName: queueName,
		tmpl:      tmpl,
	}
}

type nowPlayingData struct {
	NowPlaying liquidsoap.NowPlaying
	TrackInfo  library.TrackInfo
}

type statusJSON struct {
	Artist      string `json:"artist"`
	Title       string `json:"title"`
	ContentType string `json:"content_type"`
	IsLive      bool   `json:"is_live"`
	AlbumArt    string `json:"album_art,omitempty"`
	Album       string `json:"album,omitempty"`
}

func (h *StatusHandler) getNowPlaying() (nowPlayingData, error) {
	np, err := h.ls.OnAir()
	if err != nil {
		return nowPlayingData{}, err
	}
	ti := h.lastfm.Fetch(np.Artist, np.Title)
	return nowPlayingData{NowPlaying: np, TrackInfo: ti}, nil
}

func (h *StatusHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	data, err := h.getNowPlaying()
	if err != nil {
		slog.Error("dashboard: on_air", "err", err)
	}

	queue, err := h.ls.QueueEntries(h.queueName)
	if err != nil {
		slog.Error("dashboard: queue", "err", err)
	}

	listing, err := h.browser.List("")
	if err != nil {
		slog.Error("dashboard: library", "err", err)
	}

	uptime, _ := h.ls.Uptime()

	h.tmpl.ExecuteTemplate(w, "index.html", map[string]any{
		"NowPlaying":  data.NowPlaying,
		"TrackInfo":   data.TrackInfo,
		"Queue":       queue,
		"QueueTarget": "#queue-list",
		"Listing":     listing,
		"Breadcrumbs": library.Breadcrumbs(listing.RelPath),
		"Uptime":      uptime,
	})
}

func (h *StatusHandler) NowPlayingPartial(w http.ResponseWriter, r *http.Request) {
	data, err := h.getNowPlaying()
	if err != nil {
		slog.Warn("now-playing partial: on_air", "err", err)
	}
	if err := h.tmpl.ExecuteTemplate(w, "now-playing.html", data); err != nil {
		slog.Error("template now-playing", "err", err)
	}
}

func (h *StatusHandler) BadgePartial(w http.ResponseWriter, r *http.Request) {
	np, err := h.ls.OnAir()
	if err != nil {
		slog.Warn("badge partial: on_air", "err", err)
	}
	uptime, err := h.ls.Uptime()
	if err != nil {
		slog.Warn("badge partial: uptime", "err", err)
	}
	if err := h.tmpl.ExecuteTemplate(w, "badge", map[string]any{
		"NowPlaying": np,
		"Uptime":     uptime,
	}); err != nil {
		slog.Error("template badge", "err", err)
	}
}

func (h *StatusHandler) StatusJSON(w http.ResponseWriter, r *http.Request) {
	np, err := h.ls.OnAir()
	if err != nil {
		slog.Warn("status json: on_air", "err", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(statusJSON{})
		return
	}

	ti := h.lastfm.Fetch(np.Artist, np.Title)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statusJSON{
		Artist:      np.Artist,
		Title:       np.Title,
		ContentType: np.ContentType,
		IsLive:      np.IsLive(),
		AlbumArt:    ti.Image,
		Album:       ti.Album,
	})
}
