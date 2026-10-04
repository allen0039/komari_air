package clients

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/komari-monitor/komari/database/models"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func calibrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Client{}, &models.AgentConfig{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	return db
}

func TestNextCalibrationReset(t *testing.T) {
	for _, tc := range []struct {
		name, saved, want string
		day               int
	}{
		{"before day", "2026-10-01T12:00:00+08:00", "2026-10-02T00:00:00+08:00", 2},
		{"on day", "2026-10-02T00:00:00+08:00", "2026-11-02T00:00:00+08:00", 2},
		{"after day", "2026-10-02T22:00:00+08:00", "2026-11-02T00:00:00+08:00", 2},
		{"year rollover", "2026-12-31T12:00:00+08:00", "2027-01-31T00:00:00+08:00", 31},
		{"absent day rolls forward", "2026-01-31T12:00:00+08:00", "2026-03-01T00:00:00+08:00", 31},
		{"leap year", "2028-02-01T12:00:00+08:00", "2028-02-29T00:00:00+08:00", 29},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saved, _ := time.Parse(time.RFC3339, tc.saved)
			want, _ := time.Parse(time.RFC3339, tc.want)
			if got := nextCalibrationReset(tc.day, saved); !got.Equal(want) {
				t.Fatalf("got %s, want %s", got, want)
			}
		})
	}
}

func TestCalibrationPersistsUntilNextCycleAndCatchesUpAfterDowntime(t *testing.T) {
	db := calibrationDB(t)
	zone := time.FixedZone("panel", 8*3600)
	saved := time.Date(2026, 10, 2, 22, 0, 0, 123, zone)
	client := models.Client{UUID: "node", Token: "token", Name: "node", TrafficLimit: 800 * 1024 * 1024 * 1024, Price: 239.9}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}
	config := models.AgentConfig{UUID: client.UUID, ReportedConfig: `{"month_rotate":2}`, DesiredConfig: `{"month_rotate":1}`}
	if err := db.Create(&config).Error; err != nil {
		t.Fatal(err)
	}
	if err := saveClient(db, map[string]interface{}{"uuid": client.UUID, "traffic_used_offset": float64(71 * 1024 * 1024 * 1024)}, saved); err != nil {
		t.Fatal(err)
	}
	load := func() models.Client {
		t.Helper()
		var result models.Client
		if err := db.First(&result, "uuid = ?", client.UUID).Error; err != nil {
			t.Fatal(err)
		}
		return result
	}
	if got := load(); got.TrafficCalibrationAt == nil || !got.TrafficCalibrationAt.Equal(saved) {
		t.Fatal("calibration timestamp not persisted")
	}
	// Saving unrelated client metadata must not start a new calibration cycle.
	if err := saveClient(db, map[string]interface{}{"uuid": client.UUID, "name": "renamed"}, saved.AddDate(0, 0, 20)); err != nil {
		t.Fatal(err)
	}
	before := time.Date(2026, 11, 1, 23, 59, 59, 0, zone)
	if err := resetTrafficCalibration(db, before); err != nil {
		t.Fatal(err)
	}
	if got := load(); got.TrafficUsedOffset == 0 {
		t.Fatal("cleared before the reported reset day")
	}
	// An offline client also clears; no agent reports or notification settings are required.
	if err := resetTrafficCalibration(db, before.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if got := load(); got.TrafficUsedOffset != 0 || got.TrafficCalibrationAt != nil || got.TrafficCalibrationBaseline != nil || got.Price != client.Price || got.TrafficLimit != client.TrafficLimit {
		t.Fatal("incorrect reset", got)
	}
	if err := saveClient(db, map[string]interface{}{"uuid": client.UUID, "traffic_used_offset": float64(123)}, saved); err != nil {
		t.Fatal(err)
	}
	if err := resetTrafficCalibration(db, saved.AddDate(0, 3, 0)); err != nil {
		t.Fatal(err)
	}
	if got := load(); got.TrafficUsedOffset != 0 {
		t.Fatal("did not catch up after downtime")
	}
}

func TestCalibrationLegacyAndDisabledPolicies(t *testing.T) {
	for _, tc := range []struct {
		name, reported, desired string
		enabled                 bool
	}{
		{"reported", `{"month_rotate":2}`, "", true},
		{"desired only", "", `{"month_rotate":2}`, true},
		{"disabled overrides pending", `{"month_rotate":0}`, `{"month_rotate":2}`, false},
		{"no config", "", "", false},
		{"malformed config", `invalid`, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := calibrationDB(t)
			now := time.Date(2026, 10, 2, 22, 0, 0, 0, time.UTC)
			original := models.Client{UUID: "node", Token: "token", TrafficUsedOffset: 76235669504}
			if err := db.Create(&original).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&models.AgentConfig{UUID: "node", ReportedConfig: tc.reported, DesiredConfig: tc.desired}).Error; err != nil {
				t.Fatal(err)
			}
			if err := resetTrafficCalibration(db, now); err != nil {
				t.Fatal(err)
			}
			var client models.Client
			db.First(&client, "uuid = ?", "node")
			if client.TrafficUsedOffset != original.TrafficUsedOffset {
				t.Fatal("legacy value lost on upgrade")
			}
			if tc.enabled && (client.TrafficCalibrationAt == nil || !client.TrafficCalibrationAt.Equal(now)) {
				t.Fatal("legacy value not adopted")
			}
			if err := resetTrafficCalibration(db, now.AddDate(0, 1, 0)); err != nil {
				t.Fatal(err)
			}
			db.First(&client, "uuid = ?", "node")
			if tc.enabled && client.TrafficUsedOffset != 0 {
				t.Fatal("legacy value not cleared")
			}
			if !tc.enabled && client.TrafficUsedOffset != original.TrafficUsedOffset {
				t.Fatal("disabled calibration cleared")
			}
		})
	}
}

func TestClearingCalibrationAndProtectingTimestamp(t *testing.T) {
	db := calibrationDB(t)
	now := time.Now()
	if err := db.Create(&models.Client{UUID: "node", Token: "token", TrafficUsedOffset: 1, TrafficCalibrationAt: &now}).Error; err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"traffic_calibration_at", "traffic_calibration_baseline"} {
		if err := saveClient(db, map[string]interface{}{"uuid": "node", key: now}, now); err == nil {
			t.Fatal("client can overwrite server calibration state", key)
		}
	}
	if err := saveClient(db, map[string]interface{}{"uuid": "node", "traffic_used_offset": float64(0)}, now); err != nil {
		t.Fatal(err)
	}
	var client models.Client
	db.First(&client, "uuid = ?", "node")
	if client.TrafficUsedOffset != 0 || client.TrafficCalibrationAt != nil {
		t.Fatal("manual clear failed")
	}
	data, err := json.Marshal(client)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	json.Unmarshal(data, &fields)
	if _, ok := fields["traffic_calibration_at"]; ok {
		t.Fatal("internal timestamp exposed")
	}
}

func TestSavedTrafficTargetOverridesCounterAndContinues(t *testing.T) {
	for _, tc := range []struct {
		kind             string
		baseline, growth int64
	}{
		{"up", 100, 10}, {"down", 200, 10}, {"sum", 300, 20}, {"min", 100, 10}, {"max", 200, 10},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			db := calibrationDB(t)
			id := t.Name()
			now := time.Now()
			if err := db.Create(&models.Client{UUID: id, Token: id, TrafficLimitType: tc.kind}).Error; err != nil {
				t.Fatal(err)
			}
			agent_runtime.RecordReport(v2.Report{UUID: id, UpdatedAt: now, Network: v2.NetworkReport{TotalUp: 100, TotalDown: 200}})
			t.Cleanup(func() { agent_runtime.DeleteLatestReport(id) })
			if err := saveClient(db, map[string]interface{}{"uuid": id, "traffic_used_offset": float64(168)}, now); err != nil {
				t.Fatal(err)
			}
			load := func() models.Client {
				t.Helper()
				var c models.Client
				if err := db.First(&c, "uuid = ?", id).Error; err != nil {
					t.Fatal(err)
				}
				return c
			}
			c := load()
			if c.TrafficCalibrationBaseline == nil || *c.TrafficCalibrationBaseline != tc.baseline {
				t.Fatal("incorrect saved baseline", c)
			}
			if got := CalibratedTrafficUsed(c, 100, 200); got != 168 {
				t.Fatal("entered total was added instead of replacing usage", got)
			}
			if got := CalibratedTrafficUsed(c, 110, 210); got != 168+tc.growth {
				t.Fatal("new traffic not counted", got)
			}
			// A panel restart or listing must keep the original saved baseline.
			if err := resetTrafficCalibration(db, now.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			c = load()
			if *c.TrafficCalibrationBaseline != tc.baseline {
				t.Fatal("baseline changed on read")
			}
			agent_runtime.RecordReport(v2.Report{UUID: id, UpdatedAt: now.Add(time.Second), Network: v2.NetworkReport{TotalUp: 110, TotalDown: 210}})
			// Re-entering the same total must establish a new snapshot rather than silently doing nothing.
			if err := saveClient(db, map[string]interface{}{"uuid": id, "traffic_used_offset": float64(168)}, now.Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			c = load()
			if got := CalibratedTrafficUsed(c, 110, 210); got != 168 {
				t.Fatal("same-value recalibration failed", got)
			}
			if got := CalibratedTrafficUsed(c, 120, 220); got != 168+tc.growth {
				t.Fatal("traffic after recalibration incorrect", got)
			}
			// Neither directional samples nor history are rewritten by calibration.
			report := agent_runtime.GetLatestReport()[id]
			if report.Network.TotalUp != 110 || report.Network.TotalDown != 210 {
				t.Fatal("raw report changed")
			}
		})
	}
}

func TestPendingAndLegacyTargetsBindToFirstAvailableReport(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprint(legacy), func(t *testing.T) {
			db := calibrationDB(t)
			id := t.Name()
			now := time.Now()
			c := models.Client{UUID: id, Token: id, TrafficLimitType: "sum"}
			if legacy {
				c.TrafficUsedOffset = 168
				c.TrafficCalibrationAt = &now
			}
			if err := db.Create(&c).Error; err != nil {
				t.Fatal(err)
			}
			if !legacy {
				if err := saveClient(db, map[string]interface{}{"uuid": id, "traffic_used_offset": float64(168)}, now); err != nil {
					t.Fatal(err)
				}
			}
			var pending models.Client
			db.First(&pending, "uuid = ?", id)
			if got := CalibratedTrafficUsed(pending, 100, 200); got != 168 {
				t.Fatal("pending target double counted", got)
			}
			agent_runtime.RecordReport(v2.Report{UUID: id, UpdatedAt: now, Network: v2.NetworkReport{TotalUp: 100, TotalDown: 200}})
			t.Cleanup(func() { agent_runtime.DeleteLatestReport(id) })
			if err := resetTrafficCalibration(db, now.Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			var bound models.Client
			db.First(&bound, "uuid = ?", id)
			if bound.TrafficCalibrationBaseline == nil || *bound.TrafficCalibrationBaseline != 300 {
				t.Fatal("first report was not anchored")
			}
			if got := CalibratedTrafficUsed(bound, 100, 200); got != 168 {
				t.Fatal("legacy value double counted", got)
			}
			if got := CalibratedTrafficUsed(bound, 110, 210); got != 188 {
				t.Fatal("increment incorrect", got)
			}
		})
	}
}

func TestCalibratedTrafficSaturatesWithoutOverflow(t *testing.T) {
	baseline := int64(0)
	c := models.Client{TrafficLimitType: "sum", TrafficUsedOffset: 168, TrafficCalibrationBaseline: &baseline}
	if got := CalibratedTrafficUsed(c, math.MaxInt64, math.MaxInt64); got != math.MaxInt64 {
		t.Fatal("overflow", got)
	}
}
