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

// These are the same carrier endpoints used by MiaoMiaoWu X's current return
// route task. The agent traces them with NextTrace over TCP/80.
var defaultTargets = []Target{
	{ID: "telecom-hn-mmwx", Carrier: "telecom", Region: "湖南", Host: "hn-ct-v4.ip.zstaticcdn.com", IPFamily: v2.IPFamilyIPv4, Protocol: v2.TraceProtocolTCP, Enabled: true},
	{ID: "unicom-js-mmwx", Carrier: "unicom", Region: "江苏", Host: "js-cu-v4.ip.zstaticcdn.com", IPFamily: v2.IPFamilyIPv4, Protocol: v2.TraceProtocolTCP, Enabled: true},
	{ID: "mobile-gd-mmwx", Carrier: "mobile", Region: "广东", Host: "gd-cm-v4.ip.zstaticcdn.com", IPFamily: v2.IPFamilyIPv4, Protocol: v2.TraceProtocolTCP, Enabled: true},
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
