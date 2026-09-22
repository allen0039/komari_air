package client

import (
	"testing"

	v2 "github.com/komari-monitor/komari/protocol/v2"
)

func TestValidTraceHopsAllowsTwoCandidatesPerTTL(t *testing.T) {
	hops := make([]v2.TraceHop, 0, 60)
	for ttl := 1; ttl <= 30; ttl++ {
		hops = append(hops, v2.TraceHop{Hop: ttl}, v2.TraceHop{Hop: ttl})
	}
	if !validTraceHops(hops) {
		t.Fatal("two NextTrace candidates for each TTL should be accepted")
	}
}

func TestValidTraceHopsRejectsInvalidCandidateLayout(t *testing.T) {
	for _, hops := range [][]v2.TraceHop{
		{{Hop: 0}},
		{{Hop: 31}},
		{{Hop: 1}, {Hop: 1}, {Hop: 1}},
	} {
		if validTraceHops(hops) {
			t.Fatalf("invalid hop layout accepted: %#v", hops)
		}
	}
}
