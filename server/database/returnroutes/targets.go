package returnroutes

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/protocol/v2"
)

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
	if config.Ready() {
		stored, err := config.GetAs[[]Target]("return_route_targets")
		if err == nil {
			byID := make(map[string]Target, len(stored))
			for _, target := range stored {
				byID[target.ID] = target
			}
			for i, target := range result {
				if saved, ok := byID[target.ID]; ok {
					result[i].Host, result[i].Region, result[i].Enabled = saved.Host, saved.Region, saved.Enabled
				}
			}
		}
	}
	return result
}

var hostnamePattern = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9.-]{0,251}[a-zA-Z0-9])?$`)

// SaveTargets preserves the three carrier identities and probe protocol.
func SaveTargets(input []Target) ([]Target, error) {
	if len(input) != len(defaultTargets) {
		return nil, fmt.Errorf("exactly three targets are required")
	}
	byID := make(map[string]Target, len(input))
	for _, target := range input {
		if _, exists := byID[target.ID]; exists {
			return nil, fmt.Errorf("duplicate target %s", target.ID)
		}
		byID[target.ID] = target
	}
	result := make([]Target, len(defaultTargets))
	for i, base := range defaultTargets {
		item, ok := byID[base.ID]
		if !ok {
			return nil, fmt.Errorf("missing target %s", base.ID)
		}
		item.Host = strings.TrimSpace(item.Host)
		item.Region = strings.TrimSpace(item.Region)
		if !validTargetHost(item.Host) {
			return nil, fmt.Errorf("invalid host for %s", base.Carrier)
		}
		if len(item.Region) == 0 || len([]rune(item.Region)) > 32 {
			return nil, fmt.Errorf("invalid region for %s", base.Carrier)
		}
		base.Host, base.Region, base.Enabled = item.Host, item.Region, item.Enabled
		result[i] = base
	}
	if err := config.Set("return_route_targets", result); err != nil {
		return nil, err
	}
	return result, nil
}

func validTargetHost(host string) bool {
	if len(host) == 0 || len(host) > 253 || !hostnamePattern.MatchString(host) || strings.Contains(host, "..") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.To4() != nil && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsMulticast() && !ip.IsUnspecified()
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
	}
	return true
}

func TargetByID(id string) (Target, bool) {
	for _, target := range Targets() {
		if target.ID == id {
			return target, true
		}
	}
	return Target{}, false
}
