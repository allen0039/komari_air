package notifier

import (
	"math"
	"testing"
)

func TestCalibratedUsedByType(t *testing.T) {
	for _, tc := range []struct {
		name   string
		typeOf string
		up     int64
		down   int64
		offset int64
		want   int64
	}{
		{"sum after reinstall", "sum", 0, 0, 120, 120},
		{"sum continues", "sum", 30, 20, 120, 170},
		{"download only", "down", 30, 20, 120, 140},
		{"disabled", "max", 30, 20, 0, 30},
		{"saturates", "up", math.MaxInt64 - 2, 0, 10, math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := calibratedUsedByType(tc.typeOf, tc.up, tc.down, tc.offset); got != tc.want {
				t.Fatalf("used = %d, want %d", got, tc.want)
			}
		})
	}
}
