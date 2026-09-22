package returnroutes

import "github.com/komari-monitor/komari/protocol/v2"

type Target struct {
	ID       string           `json:"id"`
	Carrier  string           `json:"carrier"`
	Region   string           `json:"region"`
	Host     string           `json:"host"`
	IPFamily v2.IPFamily      `json:"ip_family"`
	Protocol v2.TraceProtocol `json:"protocol"`
	Enabled  bool             `json:"enabled"`
}

// The initial targets are deliberately small and replaceable. They are used as
// probe destinations, not as proof of a route label; classification still
// requires evidence from the returned path.
var defaultTargets = []Target{
	{ID: "telecom-bj-a", Carrier: "telecom", Region: "北京", Host: "202.96.199.133", IPFamily: v2.IPFamilyIPv4, Protocol: v2.TraceProtocolICMP, Enabled: true},
	{ID: "telecom-gd-a", Carrier: "telecom", Region: "广东", Host: "202.96.134.33", IPFamily: v2.IPFamilyIPv4, Protocol: v2.TraceProtocolICMP, Enabled: true},
	{ID: "unicom-sh-a", Carrier: "unicom", Region: "上海", Host: "210.22.70.3", IPFamily: v2.IPFamilyIPv4, Protocol: v2.TraceProtocolICMP, Enabled: true},
	{ID: "unicom-bj-a", Carrier: "unicom", Region: "北京", Host: "210.21.196.6", IPFamily: v2.IPFamilyIPv4, Protocol: v2.TraceProtocolICMP, Enabled: true},
	{ID: "mobile-bj-a", Carrier: "mobile", Region: "北京", Host: "221.179.155.161", IPFamily: v2.IPFamilyIPv4, Protocol: v2.TraceProtocolICMP, Enabled: true},
	{ID: "mobile-gd-a", Carrier: "mobile", Region: "广东", Host: "211.136.17.107", IPFamily: v2.IPFamilyIPv4, Protocol: v2.TraceProtocolICMP, Enabled: true},
}

func Targets() []Target {
	result := make([]Target, len(defaultTargets))
	copy(result, defaultTargets)
	return result
}

func TargetByID(id string) (Target, bool) {
	for _, target := range defaultTargets {
		if target.ID == id {
			return target, true
		}
	}
	return Target{}, false
}
