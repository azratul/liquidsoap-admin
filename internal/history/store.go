package history

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"radio-web/internal/library"
	"radio-web/internal/liquidsoap"
)

const historyPruneInterval = 24 * time.Hour

// Track representa una canción del historial.
type Track struct {
	Artist   string    `json:"artist"`
	Title    string    `json:"title"`
	Album    string    `json:"album,omitempty"`
	AlbumArt string    `json:"album_art,omitempty"`
	PlayedAt time.Time `json:"played_at"`
}

// Store gestiona el historial de reproducción en SQLite.
type Store struct {
	db        *sql.DB
	ls        *liquidsoap.Client
	lastfm    *library.LastFMClient
	interval  time.Duration
	mu        sync.Mutex
	lastTrack string // "artist|title" del último track registrado
}

// New abre (o crea) la base de datos SQLite y devuelve un Store listo.
func New(dbPath string, ls *liquidsoap.Client, lastfm *library.LastFMClient, interval time.Duration) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("history: open db: %w", err)
	}
	// SQLite no soporta escrituras concurrentes; una sola conexión es suficiente.
	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("history: migrate: %w", err)
	}
	if _, err := pruneHistory(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("history: prune: %w", err)
	}

	return &Store{
		db:       db,
		ls:       ls,
		lastfm:   lastfm,
		interval: interval,
	}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS history (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		artist    TEXT    NOT NULL,
		title     TEXT    NOT NULL,
		album     TEXT,
		album_art TEXT,
		played_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	);
	CREATE INDEX IF NOT EXISTS idx_history_played_at ON history(played_at);
	`)
	return err
}

// pruneHistory elimina las reproducciones con más de seis meses de antigüedad.
func pruneHistory(db *sql.DB) (int64, error) {
	result, err := db.Exec(`
		DELETE FROM history
		WHERE played_at < strftime('%Y-%m-%dT%H:%M:%SZ', 'now', '-6 months')
	`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Run inicia el bucle de polling. Bloqueante; llamar en una goroutine.
// Se detiene cuando ctx es cancelado.
func (s *Store) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	pruneTicker := time.NewTicker(historyPruneInterval)
	defer pruneTicker.Stop()

	// Poll inmediato al arrancar para registrar la canción actual sin esperar.
	s.poll()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.poll()
		case <-pruneTicker.C:
			s.prune()
		}
	}
}

func (s *Store) prune() {
	deleted, err := pruneHistory(s.db)
	if err != nil {
		slog.Error("history: prune", "err", err)
		return
	}
	if deleted > 0 {
		slog.Info("history: registros antiguos eliminados", "count", deleted)
	}
}

func (s *Store) poll() {
	np, err := s.ls.OnAir()
	if err != nil {
		slog.Debug("history poll: on_air", "err", err)
		return
	}
	// No registrar jingles ni metadatos incompletos: una respuesta inválida
	// del cliente Liquidsoap nunca debe llegar a la base.
	if np.Artist == "" || np.Title == "" || np.ContentType == "jingle" {
		return
	}

	key := np.Artist + "|" + np.Title

	s.mu.Lock()
	same := s.lastTrack == key
	s.mu.Unlock()

	if same {
		return
	}

	ti := s.lastfm.Fetch(np.Artist, np.Title)

	_, err = s.db.Exec(
		`INSERT INTO history (artist, title, album, album_art) VALUES (?, ?, ?, ?)`,
		np.Artist, np.Title, ti.Album, ti.Image,
	)
	if err != nil {
		slog.Error("history: insert", "err", err)
		return
	}

	s.mu.Lock()
	s.lastTrack = key
	s.mu.Unlock()

	slog.Info("history: track registrado", "artist", np.Artist, "title", np.Title)
}

// Recent devuelve los últimos n tracks del historial, del más reciente al más antiguo.
func (s *Store) Recent(n int) ([]Track, error) {
	rows, err := s.db.Query(
		`SELECT artist, title, album, album_art, played_at
		 FROM history
		 ORDER BY id DESC
		 LIMIT ?`, n,
	)
	if err != nil {
		return nil, fmt.Errorf("history: query: %w", err)
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		var t Track
		var album, albumArt sql.NullString
		if err := rows.Scan(&t.Artist, &t.Title, &album, &albumArt, &t.PlayedAt); err != nil {
			return nil, fmt.Errorf("history: scan: %w", err)
		}
		t.Album = album.String
		t.AlbumArt = albumArt.String
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// Close cierra la conexión a la base de datos.
func (s *Store) Close() error {
	return s.db.Close()
}
