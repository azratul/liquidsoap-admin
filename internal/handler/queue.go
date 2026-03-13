package handler

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"radio-web/internal/liquidsoap"
	"radio-web/internal/pathutil"
)

type QueueHandler struct {
	ls        *liquidsoap.Client
	musicRoot string
	queueName string
	tmpl      *template.Template
}

func NewQueueHandler(ls *liquidsoap.Client, musicRoot, queueName string, tmpl *template.Template) *QueueHandler {
	return &QueueHandler{
		ls:        ls,
		musicRoot: musicRoot,
		queueName: queueName,
		tmpl:      tmpl,
	}
}

func (h *QueueHandler) Add(w http.ResponseWriter, r *http.Request) {
	rawPath := r.FormValue("path")
	if rawPath == "" {
		http.Error(w, "missing path parameter", http.StatusBadRequest)
		return
	}

	safePath, err := pathutil.SafeAudioPath(h.musicRoot, rawPath)
	if err != nil {
		slog.Warn("queue add: invalid path", "path", rawPath, "err", err)
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	rid, err := h.ls.Push(h.queueName, safePath)
	if err != nil {
		slog.Error("queue add: push", "path", safePath, "err", err)
		http.Error(w, "Liquidsoap error", http.StatusInternalServerError)
		return
	}

	slog.Info("queued", "path", safePath, "rid", rid)
	h.renderQueuePartial(w, r)
}

func (h *QueueHandler) Remove(w http.ResponseWriter, r *http.Request) {
	rid := chi.URLParam(r, "rid")
	if rid == "" {
		http.Error(w, "missing rid", http.StatusBadRequest)
		return
	}

	if err := h.ls.Remove(h.queueName, rid); err != nil {
		slog.Error("queue remove", "rid", rid, "err", err)
		http.Error(w, "Liquidsoap error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *QueueHandler) Flush(w http.ResponseWriter, r *http.Request) {
	if err := h.ls.Flush(h.queueName); err != nil {
		slog.Error("queue flush", "err", err)
		http.Error(w, "Liquidsoap error", http.StatusInternalServerError)
		return
	}
	h.renderQueuePartial(w, r)
}

func (h *QueueHandler) Skip(w http.ResponseWriter, r *http.Request) {
	if err := h.ls.Skip(h.queueName); err != nil {
		slog.Error("skip", "err", err)
		http.Error(w, "Liquidsoap error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *QueueHandler) View(w http.ResponseWriter, r *http.Request) {
	entries, err := h.ls.QueueEntries(h.queueName)
	if err != nil {
		slog.Error("queue view", "err", err)
	}
	np, _ := h.ls.OnAir()
	uptime, _ := h.ls.Uptime()

	h.tmpl.ExecuteTemplate(w, "queue.html", map[string]any{
		"Queue":      entries,
		"NowPlaying": np,
		"Uptime":     uptime,
	})
}

func (h *QueueHandler) QueuePartial(w http.ResponseWriter, r *http.Request) {
	h.renderQueuePartial(w, r)
}

func (h *QueueHandler) renderQueuePartial(w http.ResponseWriter, _ *http.Request) {
	entries, err := h.ls.QueueEntries(h.queueName)
	if err != nil {
		slog.Error("queue partial", "err", err)
	}
	if err := h.tmpl.ExecuteTemplate(w, "queue-list.html", entries); err != nil {
		slog.Error("template queue-list", "err", err)
	}
}
