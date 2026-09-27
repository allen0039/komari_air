package public

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

//go:embed bundledTheme/emerald-globe-pro.zip
var bundledEmeraldGlobePro []byte

// InstallBundledThemes makes the shipped custom theme available as a regular
// installed theme. The official theme remains embedded under the "default"
// name, so users can still switch back to it from theme management.
func InstallBundledThemes() error {
	themeDir := filepath.Join(DataDir, ThemesDir, PreferredTheme)
	manifestPath := filepath.Join(themeDir, "komari-theme.json")
	if _, err := os.Stat(manifestPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect bundled theme: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(themeDir), 0o755); err != nil {
		return fmt.Errorf("create bundled theme parent: %w", err)
	}

	stagingDir, err := os.MkdirTemp(filepath.Dir(themeDir), ".emerald-globe-pro-")
	if err != nil {
		return fmt.Errorf("create bundled theme staging directory: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	archive, err := zip.NewReader(bytes.NewReader(bundledEmeraldGlobePro), int64(len(bundledEmeraldGlobePro)))
	if err != nil {
		return fmt.Errorf("open bundled theme archive: %w", err)
	}
	for _, entry := range archive.File {
		name := filepath.Clean(strings.ReplaceAll(entry.Name, "\\", "/"))
		if name == "." || name == ".." || filepath.IsAbs(name) || strings.HasPrefix(name, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("bundled theme archive contains unsafe path %q", entry.Name)
		}

		target := filepath.Join(stagingDir, name)
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create bundled theme directory %q: %w", name, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create bundled theme parent for %q: %w", name, err)
		}
		reader, err := entry.Open()
		if err != nil {
			return fmt.Errorf("open bundled theme file %q: %w", name, err)
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			reader.Close()
			return fmt.Errorf("create bundled theme file %q: %w", name, err)
		}
		_, copyErr := io.Copy(output, reader)
		closeOutputErr := output.Close()
		closeReaderErr := reader.Close()
		if copyErr != nil {
			return fmt.Errorf("extract bundled theme file %q: %w", name, copyErr)
		}
		if closeOutputErr != nil {
			return fmt.Errorf("close bundled theme file %q: %w", name, closeOutputErr)
		}
		if closeReaderErr != nil {
			return fmt.Errorf("close bundled theme archive entry %q: %w", name, closeReaderErr)
		}
	}

	if _, err := os.Stat(filepath.Join(stagingDir, "komari-theme.json")); err != nil {
		return fmt.Errorf("bundled theme archive is missing komari-theme.json: %w", err)
	}
	if err := os.RemoveAll(themeDir); err != nil {
		return fmt.Errorf("replace incomplete bundled theme: %w", err)
	}
	if err := os.Rename(stagingDir, themeDir); err != nil {
		return fmt.Errorf("activate bundled theme: %w", err)
	}
	return nil
}
