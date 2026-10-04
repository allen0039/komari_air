package notifier

import (
	"math"
	"testing"

	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/models"
)

func TestCalibratedUsedByType(t *testing.T) {
	for _, tc := range []struct {
		name     string
		typeOf   string
		up       int64
		down     int64
		target   int64
		baseline int64
		want     int64
	}{
		{"sum after reinstall", "sum", 0, 0, 120, 0, 120},
		{"sum continues", "sum", 30, 20, 120, 40, 130},
		{"download only", "down", 30, 20, 120, 10, 130},
		{"disabled", "max", 30, 20, 0, 0, 30},
		{"saturates", "up", math.MaxInt64 - 2, 0, 10, 0, math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := clients.CalibratedTrafficUsed(models.Client{TrafficLimitType: tc.typeOf, TrafficUsedOffset: tc.target, TrafficCalibrationBaseline: &tc.baseline}, tc.up, tc.down); got != tc.want {
				t.Fatalf("used = %d, want %d", got, tc.want)
			}
		})
	}
}
