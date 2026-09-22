package server

import (
	"math"
	"testing"
)

func TestParseNextTrace(t *testing.T) {
	data := []byte(`{"Hops":[[{"Success":true,"Address":{"IP":"59.43.189.201"},"Hostname":"","TTL":4,"RTT":12340000,"Geo":{"asnumber":"","country":"中国","prov":"上海","city":"上海","owner":"","isp":"中国电信","whois":"CN2-BackBone"}}],[{"Success":false,"TTL":5}]]}`)
	hops, err := parseNextTrace(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(hops) != 2 {
		t.Fatalf("got %d hops, want 2", len(hops))
	}
	if hops[0].Hop != 4 || hops[0].IP != "59.43.189.201" || hops[0].Host != "中国电信 CN2-BackBone" || hops[0].Location != "中国 上海 上海" {
		t.Fatalf("unexpected first hop: %#v", hops[0])
	}
	if math.Abs(hops[0].RTTMs-12.34) > 0.001 || hops[0].Loss != 0 {
		t.Fatalf("unexpected timing: %#v", hops[0])
	}
	if hops[1].Hop != 2 || hops[1].Loss != 100 {
		t.Fatalf("unexpected failed hop: %#v", hops[1])
	}
}
