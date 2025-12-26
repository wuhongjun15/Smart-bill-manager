package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveUploadsDirAbs(uploadsDir string) (string, error) {
	if uploadsDir == "" {
		uploadsDir = "uploads"
	}
	if filepath.IsAbs(uploadsDir) {
		return filepath.Clean(uploadsDir), nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(wd, uploadsDir)), nil
}

// resolveUploadsFilePath resolves an uploads-relative path stored in DB (e.g. "uploads/abc.png").
// to an absolute file path under uploadsDir, preventing path traversal.
func resolveUploadsFilePath(uploadsDir string, storedPath string) (string, error) {
	uploadsDirAbs, err := resolveUploadsDirAbs(uploadsDir)
	if err != nil {
		return "", err
	}
	uploadsDirAbs, err = filepath.Abs(uploadsDirAbs)
	if err != nil {
		return "", err
	}

	p := strings.TrimSpace(storedPath)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}

	// Normalize separators for prefix handling.
	p = strings.ReplaceAll(p, "\\", "/")

	// If an absolute path was stored, validate it is inside uploadsDir.
	if filepath.IsAbs(p) {
		abs := filepath.Clean(p)
		rel, err := filepath.Rel(uploadsDirAbs, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return "", fmt.Errorf("path escapes uploads dir")
		}
		return abs, nil
	}

	p = strings.TrimPrefix(p, "/")
	if strings.HasPrefix(p, "uploads/") {
		p = strings.TrimPrefix(p, "uploads/")
	}

	cleanRel := filepath.Clean(p)
	if cleanRel == "." || cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid relative path")
	}

	abs := filepath.Join(uploadsDirAbs, cleanRel)
	abs, err = filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(uploadsDirAbs, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes uploads dir")
	}

	return abs, nil
}
