package history

import (
	"bufio"
	"database/sql"
	"net"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"radio-web/internal/library"
	"radio-web/internal/liquidsoap"
)

func TestPruneHistoryKeepsLastSixMonths(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		t.Fatal(err)
	}

	for _, track := range []struct {
		title    string
		modifier string
	}{
		{title: "old", modifier: "-7 months"},
		{title: "recent", modifier: "-5 months"},
	} {
		_, err := db.Exec(`
			INSERT INTO history (artist, title, played_at)
			VALUES ('artist', ?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now', ?))
		`, track.title, track.modifier)
		if err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := pruneHistory(db)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("deleted %d rows, want 1", deleted)
	}

	var title string
	if err := db.QueryRow(`SELECT title FROM history`).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "recent" {
		t.Fatalf("remaining title %q, want recent", title)
	}
}

func TestMigrateCreatesPlayedAtIndex(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		t.Fatal(err)
	}

	var count int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'index' AND name = 'idx_history_played_at'
	`).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("index count %d, want 1", count)
	}
}

// closeConn, as a script entry or suffix, makes the fake Liquidsoap server
// close the connection (after writing the prefix, if any) instead of "END".
const closeConn = "\x00CLOSE"

// newTestStore starts a fake Liquidsoap command server whose responses follow
// script (one entry per command received) and returns a Store backed by an
// in-memory database and by a client connected to that server.
func newTestStore(t *testing.T, script ...string) *Store {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	responses := make(chan string, len(script))
	for _, s := range script {
		responses <- s
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				sc := bufio.NewScanner(conn)
				for sc.Scan() {
					select {
					case resp := <-responses:
						if data, doClose := strings.CutSuffix(resp, closeConn); doClose {
							conn.Write([]byte(data))
							return
						}
						conn.Write([]byte(resp))
					default:
						return
					}
				}
			}()
		}
	}()

	ls := liquidsoap.NewClient(ln.Addr().String())
	t.Cleanup(ls.Close)

	// Sin API key: Fetch devuelve TrackInfo{} sin tocar la red.
	lastfm := library.NewLastFMClient("", "")

	s, err := New(":memory:", ls, lastfm, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func countRows(t *testing.T, s *Store) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM history`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPollSkipsTracksWithoutArtistOrTitle(t *testing.T) {
	s := newTestStore(t,
		"The Artist||/music/a.mp3|song\nEND\n",
		"|Untitled|/music/b.mp3|song\nEND\n",
	)

	s.poll()
	s.poll()

	if n := countRows(t, s); n != 0 {
		t.Fatalf("rows %d, want 0", n)
	}
}

func TestPollDoesNotDuplicateAfterTransientFailure(t *testing.T) {
	track := "Bloc Party|Like Eating Glass|/music/a.mp3|song\nEND\n"
	s := newTestStore(t,
		track, // poll 1: se registra
		"Connection timed out.. Bye!\n"+closeConn, // poll 2: primer intento muere
		closeConn, // poll 2: el reintento también muere → error, no se registra nada
		track,     // poll 3: misma canción tras reconectar → dedupe
	)

	s.poll()
	s.poll()
	s.poll()

	if n := countRows(t, s); n != 1 {
		t.Fatalf("rows %d, want 1", n)
	}

	tracks, err := s.Recent(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 1 || tracks[0].Artist != "Bloc Party" {
		t.Fatalf("recent %+v, want single Bloc Party entry", tracks)
	}
}
