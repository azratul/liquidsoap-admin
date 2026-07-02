package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"radio-web/internal/history"
)

const (
	historyDefaultLimit = 20
	historyMaxLimit     = 100
)

type HistoryHandler struct {
	store *history.Store
}

func NewHistoryHandler(store *history.Store) *HistoryHandler {
	return &HistoryHandler{store: store}
}

func (h *HistoryHandler) Recent(w http.ResponseWriter, r *http.Request) {
	limit := historyDefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= historyMaxLimit {
			limit = n
		}
	}

	tracks, err := h.store.Recent(limit)
	if err != nil {
		slog.Error("history handler", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if tracks == nil {
		tracks = []history.Track{} // nunca null en JSON
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(tracks)
}
