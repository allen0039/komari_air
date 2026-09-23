package agentdist

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVersionUsesConfiguredDistributionDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KOMARI_AGENT_DIST_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "version"), []byte("0.1.9\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	version, err := Version()
	if err != nil {
		t.Fatal(err)
	}
	if version != "0.1.9" {
		t.Fatalf("Version() = %q, want 0.1.9", version)
	}
	if got := Path("windows", "amd64"); got != filepath.Join(dir, "komari-agent-windows-amd64.exe") {
		t.Fatalf("Path() = %q", got)
	}
}

func TestCanarySelectionKeepsOtherClientsOnStableVersion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KOMARI_AGENT_DIST_DIR", dir)
	t.Setenv("KOMARI_AGENT_CANARY_IPS", "158.178.229.51")
	if err := os.WriteFile(filepath.Join(dir, "version"), []byte("0.1.10"), 0o600); err != nil {
		t.Fatal(err)
	}
	canaryDir := filepath.Join(dir, "canary")
	if err := os.Mkdir(canaryDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(canaryDir, "version"), []byte("0.1.11"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		ip, version, path string
	}{
		{"158.178.229.51", "0.1.11", filepath.Join(canaryDir, "komari-agent-linux-arm64")},
		{"158.178.226.253", "0.1.10", filepath.Join(dir, "komari-agent-linux-arm64")},
		{"invalid", "0.1.10", filepath.Join(dir, "komari-agent-linux-arm64")},
	} {
		version, err := VersionForIP(tc.ip)
		if err != nil || version != tc.version {
			t.Fatalf("VersionForIP(%q) = %q, %v; want %q", tc.ip, version, err, tc.version)
		}
		if path := PathForIP(tc.ip, "linux", "arm64"); path != tc.path {
			t.Fatalf("PathForIP(%q) = %q; want %q", tc.ip, path, tc.path)
		}
	}
}
