package returnroutes

import (
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/internal/scheduler"
	"github.com/komari-monitor/komari/protocol/v2"
)

type Target struct {
	ID           string           `json:"id"`
	Carrier      string           `json:"carrier"`
	Region       string           `json:"region"`
	Host         string           `json:"host"`
	IPFamily     v2.IPFamily      `json:"ip_family"`
	Protocol     v2.TraceProtocol `json:"protocol"`
	Enabled      bool             `json:"enabled"`
	ProbeEnabled bool             `json:"probe_enabled"`
	SortOrder    int              `json:"sort_order"`
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
	probeEnabled := ProbeEnabled()
	for i := range result {
		result[i].ProbeEnabled = probeEnabled
	}
	if config.Ready() {
		stored, err := config.GetAs[[]Target]("return_route_targets")
		if err == nil {
			byID := make(map[string]Target, len(stored))
			for _, target := range stored {
				byID[target.ID] = target
			}
			for i, target := range result {
				if saved, ok := byID[target.ID]; ok {
					result[i].Host, result[i].Region, result[i].Enabled, result[i].SortOrder = saved.Host, saved.Region, saved.Enabled, saved.SortOrder
				}
			}
			if len(stored) == len(result) {
				sort.SliceStable(result, func(i, j int) bool { return result[i].SortOrder < result[j].SortOrder })
			}
		}
	}
	return result
}

func ProbeEnabled() bool {
	if !config.Ready() {
		return false
	}
	value, err := config.GetAs[bool]("return_route_enabled")
	return err == nil && value
}

func SaveProbeEnabled(enabled bool) error { return config.Set("return_route_enabled", enabled) }

func ScheduleTime() string {
	if !config.Ready() {
		return "04:20"
	}
	value, err := config.GetAs[string]("return_route_schedule", "04:20")
	if err != nil || !validScheduleTime(value) {
		return "04:20"
	}
	return value
}

func SaveScheduleTime(value string) error {
	if !validScheduleTime(value) {
		return fmt.Errorf("invalid schedule time")
	}
	if err := config.Set("return_route_schedule", value); err != nil {
		return err
	}
	return scheduler.AddFunc("return-routes:daily", cronForTime(value), func() { RunScheduled() })
}

func RetentionDays() int {
	if !config.Ready() {
		return 2
	}
	value, err := config.GetAs[int]("return_route_retention_days", 2)
	if err != nil || value < 1 || value > 365 {
		return 2
	}
	return value
}

func SaveRetentionDays(value int) error {
	if value < 1 || value > 365 {
		return fmt.Errorf("retention days must be between 1 and 365")
	}
	return config.Set("return_route_retention_days", value)
}

func ReloadSchedule() error {
	return scheduler.AddFunc("return-routes:daily", cronForTime(ScheduleTime()), func() { RunScheduled() })
}

func cronForTime(value string) string {
	parsed, _ := time.Parse("15:04", value)
	return fmt.Sprintf("0 %d %d * * *", parsed.Minute(), parsed.Hour())
}
func validScheduleTime(value string) bool { _, err := time.Parse("15:04", value); return err == nil }

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
	if err := SaveProbeEnabled(input[0].ProbeEnabled); err != nil {
		return nil, err
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
		base.Host, base.Region, base.Enabled, base.SortOrder = item.Host, item.Region, item.Enabled, i
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
