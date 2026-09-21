package runtimeconfig

import (
	"os"
	"path/filepath"
	"testing"

	pkgflags "github.com/komari-monitor/komari-agent/cmd/flags"
	v2 "github.com/komari-monitor/komari-agent/protocol/v2"
)

func TestManagedConfigApplyPersistsAndRejectsStaleRevision(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "managed-config.json")
	t.Setenv("AGENT_MANAGED_CONFIG", configPath)

	global := pkgflags.GlobalConfig
	original := *global
	t.Cleanup(func() { *global = original })

	global.Interval = 3
	global.MonthRotate = 0
	global.DisableAutoUpdate = false
	if err := Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	next := v2.AgentManagedConfig{
		DisableAutoUpdate:  true,
		Interval:           5,
		MonthRotate:        0,
		IncludeNics:        "eth0",
		ExcludeNics:        "docker*",
		IncludeMountpoints: "/;/data",
		MemoryIncludeCache: true,
		GetIPAddrFromNic:   true,
	}
	_, applied, changed, err := Apply(1, next)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !changed || applied.Interval != 5 || !global.DisableAutoUpdate {
		t.Fatalf("managed config did not apply: changed=%v applied=%+v flags=%+v", changed, applied, *global)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("managed config was not persisted: %v", err)
	}

	stale := next
	stale.Interval = 10
	_, _, changed, err = Apply(1, stale)
	if err != nil {
		t.Fatalf("stale Apply returned error: %v", err)
	}
	if changed {
		t.Fatal("stale revision must be ignored")
	}
	rev, current := Current()
	if rev != 1 || current.Interval != 5 {
		t.Fatalf("stale revision changed current config: rev=%d config=%+v", rev, current)
	}
}

func TestValidateManagedConfig(t *testing.T) {
	valid := v2.AgentManagedConfig{Interval: 3, MonthRotate: 1}
	if err := Validate(valid); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	invalidInterval := valid
	invalidInterval.Interval = 0
	if err := Validate(invalidInterval); err == nil {
		t.Fatal("zero interval should be rejected")
	}
	invalidRotate := valid
	invalidRotate.MonthRotate = 32
	if err := Validate(invalidRotate); err == nil {
		t.Fatal("month rotate 32 should be rejected")
	}
}
