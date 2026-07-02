package history

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
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
