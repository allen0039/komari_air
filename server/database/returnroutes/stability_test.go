package returnroutes

import (
	"github.com/komari-monitor/komari/database/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestSummaryRequiresConfirmationAndRetainsLastValid(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ReturnRouteSample{}, &models.ReturnRouteResult{}); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC()
	for i, step := range []struct {
		route   string
		ok      bool
		want    string
		stale   bool
		validAt int
	}{
		{"9929", true, "9929", false, 0},
		{"Unknown", true, "9929", true, 0},
		{"10099", true, "9929", true, 0},
		{"10099", true, "10099", false, 3},
		{"9929", false, "10099", true, 3},
		{"10099", true, "10099", false, 5},
	} {
		now := base.Add(time.Duration(i) * time.Minute)
		sample := models.ReturnRouteSample{ClientID: "test", Carrier: "unicom", RouteType: step.route, OK: step.ok, Confidence: "high", TestedAt: now}
		if err := db.Create(&sample).Error; err != nil {
			t.Fatal(err)
		}
		if err := aggregateClientCarrier(db, "test", "unicom", now); err != nil {
			t.Fatal(err)
		}
		var row models.ReturnRouteResult
		if err := db.First(&row).Error; err != nil {
			t.Fatal(err)
		}
		if row.RouteType != step.want || row.Stale != step.stale || !row.TestedAt.Equal(base.Add(time.Duration(step.validAt)*time.Minute)) {
			t.Fatalf("step %d: %+v", i, row)
		}
	}
}

func TestClearResultsKeepsLogsButDiscardsOldEvidence(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ReturnRouteSample{}, &models.ReturnRouteResult{}, &models.Client{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Client{UUID: "node", Token: "token", Name: "Node"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ReturnRouteSample{ClientID: "node", Carrier: "unicom"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ReturnRouteResult{ClientID: "node", Carrier: "unicom"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := clearResults(db); err != nil {
		t.Fatal(err)
	}
	var sample models.ReturnRouteSample
	if err := db.First(&sample).Error; err != nil || !sample.Archived {
		t.Fatalf("history not archived: %+v err=%v", sample, err)
	}
	var summaries int64
	if err := db.Model(&models.ReturnRouteResult{}).Count(&summaries).Error; err != nil || summaries != 0 {
		t.Fatalf("summary remains: count=%d err=%v", summaries, err)
	}
	if err := db.Create(&models.ReturnRouteSample{ClientID: "node", Carrier: "unicom", RouteType: "10099", OK: true, Confidence: "high", TestedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := aggregateClientCarrier(db, "node", "unicom", time.Now()); err != nil {
		t.Fatal(err)
	}
	var result models.ReturnRouteResult
	if err := db.First(&result).Error; err != nil || result.RouteType != "10099" {
		t.Fatalf("new result used old evidence: %+v err=%v", result, err)
	}
	if err := clearLogs(db); err != nil {
		t.Fatal(err)
	}
	var logs int64
	if err := db.Model(&models.ReturnRouteSample{}).Where("hidden = ?", false).Count(&logs).Error; err != nil || logs != 0 {
		t.Fatalf("visible logs remain: count=%d err=%v", logs, err)
	}
	if err := db.Model(&models.ReturnRouteSample{}).Count(&logs).Error; err != nil || logs != 2 {
		t.Fatalf("confirmation evidence was deleted: count=%d err=%v", logs, err)
	}
	if err := db.Model(&models.ReturnRouteResult{}).Count(&summaries).Error; err != nil || summaries != 1 {
		t.Fatalf("clearing logs changed results: count=%d err=%v", summaries, err)
	}
	var clients int64
	if err := db.Model(&models.Client{}).Count(&clients).Error; err != nil || clients != 1 {
		t.Fatalf("client changed: count=%d err=%v", clients, err)
	}
}
