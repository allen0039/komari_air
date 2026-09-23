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
