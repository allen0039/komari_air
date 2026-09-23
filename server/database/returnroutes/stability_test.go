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
