package library

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"radio-web/internal/pathutil"
)

// Entry represents an item in the music library, either a directory or an audio file.
type Entry struct {
	Name  string
	Path  string // path related to MusicRoot (ex: "Rock/80s/track.mp3")
	IsDir bool
}

// DirListing is the result of listing a directory
type DirListing struct {
	RelPath string  // path to MusicRoot (ex: "Rock/80s")
	Entries []Entry // directories first, then files
}

// Browser manages access to the music library on disk
type Browser struct {
	musicRoot string
}

func NewBrowser(musicRoot string) *Browser {
	return &Browser{musicRoot: musicRoot}
}

// List returns the contents of the directory at userPath, relative to musicRoot.
func (b *Browser) List(userPath string) (*DirListing, error) {
	absPath, err := pathutil.SafeDirPath(b.musicRoot, userPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}

	var dirs, files []Entry

	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}

		relPath := pathutil.RelPath(b.musicRoot, filepath.Join(absPath, e.Name()))

		if e.IsDir() {
			dirs = append(dirs, Entry{
				Name:  e.Name(),
				Path:  relPath,
				IsDir: true,
			})
		} else if pathutil.IsAudioFile(e.Name()) {
			files = append(files, Entry{
				Name:  e.Name(),
				Path:  relPath,
				IsDir: false,
			})
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	relListing := pathutil.RelPath(b.musicRoot, absPath)
	if relListing == "." {
		relListing = ""
	}

	return &DirListing{
		RelPath: relListing,
		Entries: append(dirs, files...),
	}, nil
}

// Search looks for audio files in the entire music library that match the query (case-insensitive, substring).
func (b *Browser) Search(query string) ([]Entry, error) {
	q := strings.ToLower(query)
	var results []Entry

	err := filepath.WalkDir(b.musicRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if !pathutil.IsAudioFile(d.Name()) {
			return nil
		}
		if strings.Contains(strings.ToLower(d.Name()), q) {
			rel := pathutil.RelPath(b.musicRoot, path)
			results = append(results, Entry{
				Name:  d.Name(),
				Path:  rel,
				IsDir: false,
			})
		}
		return nil
	})

	return results, err
}

// Breadcrumbs generates a list of entries representing the path hierarchy for a given relative path.
func Breadcrumbs(relPath string) []Entry {
	if relPath == "" {
		return nil
	}
	parts := strings.Split(relPath, string(os.PathSeparator))
	crumbs := make([]Entry, 0, len(parts))
	for i, part := range parts {
		crumbs = append(crumbs, Entry{
			Name:  part,
			Path:  strings.Join(parts[:i+1], string(os.PathSeparator)),
			IsDir: true,
		})
	}
	return crumbs
}
