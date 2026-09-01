package project

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// downloadZip fetches url's full body into memory.
func downloadZip(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s failed: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// extractZip extracts every entry in a zip archive's raw bytes into dest,
// which must already exist.
func extractZip(data []byte, dest string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

	cleanDest := filepath.Clean(dest)
	for _, f := range r.File {
		path := filepath.Join(dest, f.Name)
		if path != cleanDest && !strings.HasPrefix(path, cleanDest+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in archive: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		if err := extractZipFile(f, path); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode()|0600)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

// singleTopLevelDir returns the sole entry inside dir if there's exactly
// one, else dir itself — GitHub's codeload archives always wrap everything
// in one "<repo>-<branch>/" folder, so this unwraps that.
func singleTopLevelDir(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(dir, entries[0].Name()), nil
	}
	return dir, nil
}

// downloadAndExtractZip downloads url and extracts it to a fresh temp
// directory, returning the path to its content — unwrapped one level if
// the archive has a single top-level folder (see singleTopLevelDir). The
// caller is responsible for removing the returned directory's parent when
// done.
func downloadAndExtractZip(url string) (string, error) {
	data, err := downloadZip(url)
	if err != nil {
		return "", err
	}

	dest, err := os.MkdirTemp("", "jcore-download-*")
	if err != nil {
		return "", err
	}

	if err := extractZip(data, dest); err != nil {
		os.RemoveAll(dest)
		return "", err
	}

	unwrapped, err := singleTopLevelDir(dest)
	if err != nil {
		os.RemoveAll(dest)
		return "", err
	}
	return unwrapped, nil
}
