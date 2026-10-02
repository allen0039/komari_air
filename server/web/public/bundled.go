package public

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed bundledTheme/emerald-globe-pro.zip
var bundledEmeraldGlobePro []byte

// InstallBundledThemes makes the shipped custom theme available as a regular
// installed theme. The official theme remains embedded under the "default"
// name, so users can still switch back to it from theme management.
func InstallBundledThemes() error {
	themeDir := filepath.Join(DataDir, ThemesDir, PreferredTheme)
	archive, err := zip.NewReader(bytes.NewReader(bundledEmeraldGlobePro), int64(len(bundledEmeraldGlobePro)))
	if err != nil {
		return fmt.Errorf("open bundled theme archive: %w", err)
	}
	manifestFile, err := archive.Open("komari-theme.json")
	if err != nil {
		return fmt.Errorf("open bundled theme manifest: %w", err)
	}
	var shipped themeVersionManifest
	decodeErr := json.NewDecoder(manifestFile).Decode(&shipped)
	manifestFile.Close()
	if decodeErr != nil || shipped.Short != PreferredTheme {
		return fmt.Errorf("invalid bundled theme manifest")
	}
	manifestPath := filepath.Join(themeDir, "komari-theme.json")
	if content, err := os.ReadFile(manifestPath); err == nil {
		var installed themeVersionManifest
		if json.Unmarshal(content, &installed) != nil || installed.Short != shipped.Short || !themeVersionOlder(installed.Version, shipped.Version) {
			return nil
		}
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
	// Keep the old files available for rollback until the new directory is active.
	previousDir := stagingDir + "-previous"
	if err := os.Rename(themeDir, previousDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stage previous bundled theme: %w", err)
	}
	if err := os.Rename(stagingDir, themeDir); err != nil {
		if restoreErr := os.Rename(previousDir, themeDir); restoreErr != nil && !os.IsNotExist(restoreErr) {
			return fmt.Errorf("activate bundled theme: %w (restore failed: %v)", err, restoreErr)
		}
		return fmt.Errorf("activate bundled theme: %w", err)
	}
	if err := os.RemoveAll(previousDir); err != nil {
		return fmt.Errorf("clean previous bundled theme: %w", err)
	}
	return nil
}

type themeVersionManifest struct {
	Short   string `json:"short"`
	Version string `json:"version"`
}

// Only upgrade known numeric releases. Preserve unknown, customized or newer versions.
func themeVersionOlder(installed, shipped string) bool {
	parse := func(version string) ([3]uint64, bool) {
		var numbers [3]uint64
		parts := strings.Split(version, ".")
		if len(parts) != len(numbers) {
			return numbers, false
		}
		for i, part := range parts {
			if part == "" {
				return numbers, false
			}
			for _, c := range part {
				if c < '0' || c > '9' {
					return numbers, false
				}
			}
			value, err := strconv.ParseUint(part, 10, 64)
			if err != nil {
				return numbers, false
			}
			numbers[i] = value
		}
		return numbers, true
	}
	old, validOld := parse(installed)
	new, validNew := parse(shipped)
	if !validOld || !validNew {
		return false
	}
	for i := range old {
		if old[i] != new[i] {
			return old[i] < new[i]
		}
	}
	return false
}
