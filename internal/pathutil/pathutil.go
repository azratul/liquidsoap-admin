package pathutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var allowedExtensions = map[string]bool{
	".mp3":  true,
	".flac": true,
	".ogg":  true,
	".wav":  true,
	".aac":  true,
	".m4a":  true,
	".opus": true,
}

// SafeAudioPath validates that userPath is within base, exists, is a file, and has an allowed audio extension.
func SafeAudioPath(base, userPath string) (string, error) {
	clean := filepath.Clean(filepath.Join(base, userPath))
	baseClean := filepath.Clean(base)

	if !strings.HasPrefix(clean, baseClean+string(os.PathSeparator)) && clean != baseClean {
		return "", fmt.Errorf("path outside allowed root: %q", userPath)
	}

	info, err := os.Stat(clean)
	if err != nil {
		return "", fmt.Errorf("path not found: %q", clean)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, expected a file: %q", clean)
	}

	ext := strings.ToLower(filepath.Ext(clean))
	if !allowedExtensions[ext] {
		return "", fmt.Errorf("extension not allowed: %q", ext)
	}

	return clean, nil
}

// SafeDirPath validates that userPath is within base, exists, and is a directory. If userPath is empty or "/", it returns base.
func SafeDirPath(base, userPath string) (string, error) {
	var clean string
	if userPath == "" || userPath == "/" {
		clean = filepath.Clean(base)
	} else {
		clean = filepath.Clean(filepath.Join(base, userPath))
	}
	baseClean := filepath.Clean(base)

	if !strings.HasPrefix(clean, baseClean) {
		return "", fmt.Errorf("path outside allowed root: %q", userPath)
	}

	info, err := os.Stat(clean)
	if err != nil {
		return "", fmt.Errorf("directory not found: %q", clean)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %q", clean)
	}

	return clean, nil
}

// IsAudioFile returns true if the file has an allowed audio extension.
func IsAudioFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return allowedExtensions[ext]
}

// RelPath returns the path of absPath relative to base. If absPath is not within base, it returns absPath.
func RelPath(base, absPath string) string {
	rel, err := filepath.Rel(base, absPath)
	if err != nil {
		return absPath
	}
	return rel
}
