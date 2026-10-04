package clients

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
	"gorm.io/gorm"
)

// ResetTrafficCalibration runs independently of online agents and notification settings.
func ResetTrafficCalibration() error {
	return resetTrafficCalibration(dbcore.GetDBInstance(), time.Now())
}

func resetTrafficCalibration(db *gorm.DB, now time.Time) error {
	var calibrated []models.Client
	if err := db.Select("uuid", "traffic_used_offset", "traffic_calibration_at", "traffic_calibration_baseline", "traffic_limit_type").Where("traffic_used_offset > 0").Find(&calibrated).Error; err != nil {
		return err
	}
	if len(calibrated) == 0 {
		return nil
	}
	ids := make([]string, 0, len(calibrated))
	for _, client := range calibrated {
		ids = append(ids, client.UUID)
	}
	var configs []models.AgentConfig
	if err := db.Where("uuid IN ?", ids).Find(&configs).Error; err != nil {
		return err
	}
	resetDays := make(map[string]int, len(configs))
	for _, config := range configs {
		// A pending desired configuration has not necessarily taken effect on the agent.
		raw := config.ReportedConfig
		if raw == "" {
			raw = config.DesiredConfig
		}
		var managed v2.AgentManagedConfig
		if json.Unmarshal([]byte(raw), &managed) == nil && managed.MonthRotate >= 1 && managed.MonthRotate <= 31 {
			resetDays[config.UUID] = managed.MonthRotate
		}
	}
	reports := agent_runtime.GetLatestReport()
	for _, client := range calibrated {
		// Bind legacy or offline calibrations once, preserving the entered target.
		if client.TrafficCalibrationBaseline == nil {
			if report := reports[client.UUID]; report != nil {
				baseline := TrafficUsedByType(client.TrafficLimitType, report.Network.TotalUp, report.Network.TotalDown)
				query := db.Model(&models.Client{}).Where("uuid = ? AND traffic_used_offset = ? AND traffic_calibration_baseline IS NULL", client.UUID, client.TrafficUsedOffset)
				if client.TrafficCalibrationAt == nil {
					query = query.Where("traffic_calibration_at IS NULL")
				} else {
					query = query.Where("traffic_calibration_at = ?", client.TrafficCalibrationAt)
				}
				if err := query.Update("traffic_calibration_baseline", baseline).Error; err != nil {
					return err
				}
			}
		}
		day := resetDays[client.UUID]
		if day == 0 {
			continue
		}
		query := db.Model(&models.Client{}).Where("uuid = ? AND traffic_used_offset = ?", client.UUID, client.TrafficUsedOffset)
		if client.TrafficCalibrationAt == nil {
			// Adopt legacy values in the current cycle without clearing them on upgrade.
			if err := query.Where("traffic_calibration_at IS NULL").Update("traffic_calibration_at", now.UTC()).Error; err != nil {
				return err
			}
			continue
		}
		expires := nextCalibrationReset(day, client.TrafficCalibrationAt.In(now.Location()))
		if now.Before(expires) {
			continue
		}
		// Compare the saved timestamp too, so a concurrently re-saved value is preserved.
		if err := query.Where("traffic_calibration_at = ?", client.TrafficCalibrationAt).Updates(map[string]any{
			"traffic_used_offset": int64(0), "traffic_calibration_at": nil, "traffic_calibration_baseline": nil,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// Match the agent: a reset day absent from a month rolls forward to the next month's first day.
func nextCalibrationReset(day int, saved time.Time) time.Time {
	boundary := func(year int, month time.Month) time.Time {
		nextMonth := time.Date(year, month+1, 1, 0, 0, 0, 0, saved.Location())
		if day > nextMonth.AddDate(0, 0, -1).Day() {
			return nextMonth
		}
		return time.Date(year, month, day, 0, 0, 0, 0, saved.Location())
	}
	next := boundary(saved.Year(), saved.Month())
	if !next.After(saved) {
		next = boundary(saved.Year(), saved.Month()+1)
	}
	return next
}

// TrafficUsedByType selects the quota counter without changing either directional sample.
func TrafficUsedByType(kind string, up, down int64) int64 {
	if up < 0 {
		up = 0
	}
	if down < 0 {
		down = 0
	}
	switch strings.ToLower(kind) {
	case "up":
		return up
	case "down":
		return down
	case "min":
		if up < down {
			return up
		}
		return down
	case "sum":
		if up > math.MaxInt64-down {
			return math.MaxInt64
		}
		return up + down
	default:
		if up > down {
			return up
		}
		return down
	}
}

// CalibratedTrafficUsed starts at the saved target and adds only subsequent usage.
func CalibratedTrafficUsed(client models.Client, up, down int64) int64 {
	raw := TrafficUsedByType(client.TrafficLimitType, up, down)
	if client.TrafficUsedOffset <= 0 {
		return raw
	}
	if client.TrafficCalibrationBaseline == nil {
		return client.TrafficUsedOffset
	}
	delta := raw - *client.TrafficCalibrationBaseline
	if delta < 0 {
		delta = 0
	}
	if delta > math.MaxInt64-client.TrafficUsedOffset {
		return math.MaxInt64
	}
	return client.TrafficUsedOffset + delta
}
