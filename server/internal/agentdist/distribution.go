package agentdist

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultDir = "/app/agent-dist"

// Dir returns the directory containing the panel-managed Agent artifacts.
func Dir() string {
	if configured := strings.TrimSpace(os.Getenv("KOMARI_AGENT_DIST_DIR")); configured != "" {
		return configured
	}
	return defaultDir
}

// Version returns the version embedded in the panel-managed Agent bundle.
func Version() (string, error) {
	data, err := os.ReadFile(filepath.Join(Dir(), "version"))
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(data))
	if version == "" {
		return "", fmt.Errorf("agent build version is empty")
	}
	return version, nil
}

// Path returns the path to one platform-specific Agent binary.
func Path(goos, goarch string) string {
	name := "komari-agent-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return filepath.Join(Dir(), name)
}
