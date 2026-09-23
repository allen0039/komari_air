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
		{"dmit japan ctg gia", Target{Carrier: "telecom"}, []v2.TraceHop{{ASN: "906"}, {ASN: "AS23764"}, {IP: "59.43.189.201"}}, "CTG GIA"},
		{"bwh unicom 9929", Target{Carrier: "unicom"}, []v2.TraceHop{{ASN: "25820"}, {ASN: "10099"}, {ASN: "9929"}, {ASN: "4837"}}, "9929"},
		{"bwh mobile cmin2", Target{Carrier: "mobile"}, []v2.TraceHop{{ASN: "58807"}, {ASN: "9808"}}, "CMIN2"},
		{"dmit malibu cn2 gia", Target{Carrier: "telecom"}, []v2.TraceHop{{ASN: "906"}, {ASN: "4134"}, {IP: "59.43.182.106"}}, "CN2 GIA"},
		{"yunyou mobile 10099", Target{Carrier: "mobile"}, []v2.TraceHop{{ASN: "10099"}, {ASN: "9808"}}, "10099"},
		{"legend sg first backbone 163", Target{Carrier: "unicom"}, []v2.TraceHop{{ASN: "216211"}, {ASN: "4134"}, {ASN: "4837"}}, "163"},
		{"bage unicom 4837", Target{Carrier: "unicom"}, []v2.TraceHop{{ASN: "26042"}, {ASN: "6461"}, {ASN: "4837"}}, "4837"},
		{"novix mobile cmi", Target{Carrier: "mobile"}, []v2.TraceHop{{ASN: "9808"}}, "CMI"},
		{"unrecognized 4808", Target{Carrier: "unicom"}, []v2.TraceHop{{ASN: "4808"}}, "Unknown"},
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

func TestTargetsMatchMiaoMiaoWuX(t *testing.T) {
	counts := map[string]int{}
	wants := map[string]string{
		"telecom": "hn-ct-v4.ip.zstaticcdn.com",
		"unicom":  "js-cu-v4.ip.zstaticcdn.com",
		"mobile":  "gd-cm-v4.ip.zstaticcdn.com",
	}
	for _, target := range Targets() {
		if target.Enabled {
			counts[target.Carrier]++
			if target.Host != wants[target.Carrier] {
				t.Fatalf("carrier %s target = %s, want %s", target.Carrier, target.Host, wants[target.Carrier])
			}
			if target.Protocol != v2.TraceProtocolTCP {
				t.Fatalf("carrier %s protocol = %s, want tcp", target.Carrier, target.Protocol)
			}
		}
	}
	for _, carrier := range []string{"telecom", "unicom", "mobile"} {
		if counts[carrier] != 1 {
			t.Fatalf("carrier %s has %d enabled targets", carrier, counts[carrier])
		}
	}
}

func TestCUIIMetadataWithoutASN(t *testing.T) {
	for _, tc := range []struct{ name, asn, host, want string }{
		{"BWH missing ASN", "", "chinaunicom.cn 联通 CUII 中国联通 CNC-BACKBONE", "9929"},
		{"explicit ASN wins", "4837", "CUII", "10099"},
		{"generic CNC is insufficient", "", "中国联通 CNC-BACKBONE", "10099"},
		{"substring is insufficient", "", "not-cuii", "10099"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hops := []v2.TraceHop{{Hop: 4, IP: "162.219.85.5", ASN: "10099"}, {Hop: 7, IP: "210.78.24.42", ASN: tc.asn, Host: tc.host}, {Hop: 8, IP: "219.158.45.73", ASN: "4837"}}
			got, _, _ := classify(Target{}, hops)
			if got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
