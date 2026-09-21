package runtimeconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	pkgflags "github.com/komari-monitor/komari-agent/cmd/flags"
	"github.com/komari-monitor/komari-agent/monitoring/netstatic"
	unit "github.com/komari-monitor/komari-agent/monitoring/unit"
	v2 "github.com/komari-monitor/komari-agent/protocol/v2"
)

type persistedConfig struct {
	Revision uint64                `json:"revision"`
	Config   v2.AgentManagedConfig `json:"config"`
}

var (
	mu       sync.RWMutex
	revision uint64
	current  v2.AgentManagedConfig
	path     string
	flags    = pkgflags.GlobalConfig
)

func fromFlags() v2.AgentManagedConfig {
	return v2.AgentManagedConfig{
		DisableAutoUpdate:  flags.DisableAutoUpdate,
		Interval:           flags.Interval,
		MonthRotate:        flags.MonthRotate,
		IncludeNics:        flags.IncludeNics,
		ExcludeNics:        flags.ExcludeNics,
		IncludeMountpoints: flags.IncludeMountpoints,
		MemoryIncludeCache: flags.MemoryIncludeCache,
		GetIPAddrFromNic:   flags.GetIpAddrFromNic,
	}
}

func applyToFlags(config v2.AgentManagedConfig) {
	flags.DisableAutoUpdate = config.DisableAutoUpdate
	flags.Interval = config.Interval
	flags.MonthRotate = config.MonthRotate
	flags.IncludeNics = config.IncludeNics
	flags.ExcludeNics = config.ExcludeNics
	flags.IncludeMountpoints = config.IncludeMountpoints
	flags.MemoryIncludeCache = config.MemoryIncludeCache
	flags.GetIpAddrFromNic = config.GetIPAddrFromNic
}

func defaultPath() string {
	if configured := os.Getenv("AGENT_MANAGED_CONFIG"); configured != "" {
		return configured
	}
	exe, err := os.Executable()
	if err != nil {
		return "managed-config.json"
	}
	return filepath.Join(filepath.Dir(exe), "managed-config.json")
}

func Validate(config v2.AgentManagedConfig) error {
	if config.Interval < 1 || config.Interval > 3600 {
		return fmt.Errorf("interval must be between 1 and 3600 seconds")
	}
	if config.MonthRotate < 0 || config.MonthRotate > 31 {
		return fmt.Errorf("month_rotate must be between 0 and 31")
	}
	if len(config.IncludeNics) > 2048 || len(config.ExcludeNics) > 2048 || len(config.IncludeMountpoints) > 4096 {
		return fmt.Errorf("managed config text field is too long")
	}
	return nil
}

func Initialize() error {
	mu.Lock()
	defer mu.Unlock()
	path = defaultPath()
	current = fromFlags()
	if current.Interval < 1 {
		current.Interval = 3
		applyToFlags(current)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var saved persistedConfig
	if err := json.Unmarshal(data, &saved); err != nil {
		return fmt.Errorf("parse managed config: %w", err)
	}
	if err := Validate(saved.Config); err != nil {
		return fmt.Errorf("validate managed config: %w", err)
	}
	revision = saved.Revision
	current = saved.Config
	applyToFlags(current)
	return nil
}

func Current() (uint64, v2.AgentManagedConfig) {
	mu.RLock()
	defer mu.RUnlock()
	return revision, current
}

func persistLocked(next persistedConfig) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func Apply(nextRevision uint64, next v2.AgentManagedConfig) (v2.AgentManagedConfig, v2.AgentManagedConfig, bool, error) {
	if err := Validate(next); err != nil {
		return v2.AgentManagedConfig{}, v2.AgentManagedConfig{}, false, err
	}
	mu.Lock()
	if nextRevision <= revision {
		old := current
		mu.Unlock()
		return old, old, false, nil
	}
	oldRevision, old := revision, current
	applyToFlags(next)
	if err := persistLocked(persistedConfig{Revision: nextRevision, Config: next}); err != nil {
		applyToFlags(old)
		revision, current = oldRevision, old
		mu.Unlock()
		return old, old, false, err
	}
	revision, current = nextRevision, next
	mu.Unlock()

	if next.MonthRotate == 0 {
		_ = netstatic.Stop()
	} else {
		_ = netstatic.StartOrContinue()
		nics, err := unit.InterfaceList()
		if err == nil {
			_ = netstatic.SetNewConfig(netstatic.NetStaticConfig{Nics: nics})
		}
	}
	return old, next, true, nil
}
