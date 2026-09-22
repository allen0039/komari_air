package returnroutes

import (
	"testing"

	v2 "github.com/komari-monitor/komari/protocol/v2"
)

func TestClassifyUsesPathEvidence(t *testing.T) {
	tests := []struct {
		name   string
		target Target
		hops   []v2.TraceHop
		want   string
	}{
		{"telecom cn2 gia", Target{Carrier: "telecom"}, []v2.TraceHop{{Host: "cn2-gia.example"}}, "CN2 GIA"},
		{"unicom 9929", Target{Carrier: "unicom"}, []v2.TraceHop{{ASN: "9929"}}, "9929"},
		{"mobile cmi", Target{Carrier: "mobile"}, []v2.TraceHop{{Host: "cmi.example"}}, "CMI"},
		{"unknown", Target{Carrier: "telecom"}, []v2.TraceHop{{IP: "192.0.2.1"}}, "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, _ := classify(tt.target, tt.hops)
			if got != tt.want {
				t.Fatalf("classify() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTargetsHaveTwoEnabledTargetsPerCarrier(t *testing.T) {
	counts := map[string]int{}
	for _, target := range Targets() {
		if target.Enabled {
			counts[target.Carrier]++
		}
	}
	for _, carrier := range []string{"telecom", "unicom", "mobile"} {
		if counts[carrier] < 2 {
			t.Fatalf("carrier %s has %d enabled targets", carrier, counts[carrier])
		}
	}
}
