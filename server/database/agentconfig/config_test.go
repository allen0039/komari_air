package agentconfig

import (
	"testing"

	v2 "github.com/komari-monitor/komari/protocol/v2"
)

func TestValidate(t *testing.T) {
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
	invalidRotate.MonthRotate = -1
	if err := Validate(invalidRotate); err == nil {
		t.Fatal("negative month rotate should be rejected")
	}
}
