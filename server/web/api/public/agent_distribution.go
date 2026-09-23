package public

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/internal/agentdist"
)

var safeAgentPart = regexp.MustCompile(`^[a-z0-9_-]+$`)

func ServeAgentInstallSH(c *gin.Context) {
	c.File(filepath.Join(agentdist.Dir(), "install.sh"))
}

func ServeAgentInstallPS1(c *gin.Context) {
	c.File(filepath.Join(agentdist.Dir(), "install.ps1"))
}

func AgentVersion(c *gin.Context) {
	version, err := agentdist.VersionForIP(c.ClientIP())
	if err != nil {
		c.String(http.StatusNotFound, "agent build unavailable")
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(version))
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
	full := agentdist.PathForIP(c.ClientIP(), goos, goarch)
	if _, err := os.Stat(full); err != nil {
		c.String(http.StatusNotFound, "agent build unavailable")
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+filepath.Base(full)+`"`)
	c.File(full)
}
