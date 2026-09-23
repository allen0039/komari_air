package agentdist

import (
	"fmt"
	"net/netip"
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
	return versionAt(Dir())
}

func versionAt(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "version"))
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(data))
	if version == "" {
		return "", fmt.Errorf("agent build version is empty")
	}
	return version, nil
}

// CanaryForIP reports whether this exact client IP is opted into the Agent
// canary. The public version and binary endpoints use the same selection.
func CanaryForIP(ip string) bool {
	clientIP, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return false
	}
	for _, entry := range strings.Split(os.Getenv("KOMARI_AGENT_CANARY_IPS"), ",") {
		candidate, err := netip.ParseAddr(strings.TrimSpace(entry))
		if err == nil && candidate == clientIP {
			return true
		}
	}
	return false
}

func VersionForIP(ip string) (string, error) {
	if CanaryForIP(ip) {
		return versionAt(filepath.Join(Dir(), "canary"))
	}
	return Version()
}

// Path returns the path to one platform-specific Agent binary.
func Path(goos, goarch string) string {
	return pathAt(Dir(), goos, goarch)
}

func PathForIP(ip, goos, goarch string) string {
	if CanaryForIP(ip) {
		return pathAt(filepath.Join(Dir(), "canary"), goos, goarch)
	}
	return Path(goos, goarch)
}

func pathAt(dir, goos, goarch string) string {
	name := "komari-agent-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name)
}
