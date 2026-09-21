package public

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var safeAgentPart = regexp.MustCompile(`^[a-z0-9_-]+$`)

func agentDistDir() string {
	if configured := strings.TrimSpace(os.Getenv("KOMARI_AGENT_DIST_DIR")); configured != "" {
		return configured
	}
	return "/app/agent-dist"
}

func ServeAgentInstallSH(c *gin.Context) {
	c.File(filepath.Join(agentDistDir(), "install.sh"))
}

func ServeAgentInstallPS1(c *gin.Context) {
	c.File(filepath.Join(agentDistDir(), "install.ps1"))
}

func AgentVersion(c *gin.Context) {
	data, err := os.ReadFile(filepath.Join(agentDistDir(), "version"))
	if err != nil {
		c.String(http.StatusNotFound, "agent build unavailable")
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", data)
}

func DownloadAgent(c *gin.Context) {
	goos := strings.ToLower(strings.TrimSpace(c.Param("os")))
	goarch := strings.ToLower(strings.TrimSpace(c.Param("arch")))
	if !safeAgentPart.MatchString(goos) || !safeAgentPart.MatchString(goarch) {
		c.String(http.StatusBadRequest, "invalid target")
		return
	}
	switch goos {
	case "linux", "darwin", "windows":
	default:
		c.String(http.StatusNotFound, "unsupported operating system")
		return
	}
	switch goarch {
	case "amd64", "arm64":
	default:
		c.String(http.StatusNotFound, "unsupported architecture")
		return
	}
	name := "komari-agent-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	full := filepath.Join(agentDistDir(), name)
	if _, err := os.Stat(full); err != nil {
		c.String(http.StatusNotFound, "agent build unavailable")
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.File(full)
}
