package returnroutes

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
	"gorm.io/gorm"
)

var scheduleMu sync.Mutex

func classify(target Target, hops []v2.TraceHop) (string, string, string) {
	var evidence []string
	for _, hop := range hops {
		evidence = append(evidence, strings.ToLower(hop.Host), strings.ToLower(hop.IP), strings.ToLower(hop.ASN))
	}
	joined := strings.Join(evidence, " ")
	// Normalize route probe spellings (AS4134, China Telecom, cmcc, etc.).
	joined = strings.NewReplacer("_", " ", "-", " ", ".", " ", "/", " ").Replace(joined)
	type match struct {
		label string
		token string
	}
	matches := []match{}
	switch target.Carrier {
	case "telecom":
		matches = []match{{"CN2 GIA", "cn2 gia"}, {"CN2 GIA", "cn2 gia"}, {"CN2 GT", "cn2 gt"}, {"163", "as4134"}, {"163", "4134"}, {"163", "chinanet"}, {"163", "telecom"}}
	case "unicom":
		matches = []match{{"9929", "as9929"}, {"9929", "9929"}, {"4837", "as4837"}, {"4837", "4837"}, {"4837", "chinaunicom"}, {"4837", "unicom"}}
	case "mobile":
		matches = []match{{"CMIN2", "cmin2"}, {"CMIN2", "as58453"}, {"CMI", "as9808"}, {"CMI", "9808"}, {"CMI", "cmi"}, {"CMI", "cmnet"}, {"CMI", "chinamobile"}, {"CMI", "cmcc"}, {"CMI", "mobile"}}
	}
	for _, candidate := range matches {
		if strings.Contains(joined, candidate.token) {
			return candidate.label, "medium", "matched path evidence: " + candidate.token
		}
	}
	return "Unknown", "low", "no stable carrier path signature was found"
}

func SaveResult(clientID string, result v2.NextTraceResult) error {
	target, ok := TargetByID(result.TargetID)
	if !ok {
		return fmt.Errorf("unknown return route target %q", result.TargetID)
	}
	now := result.FinishedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	routeType, confidence, reason := classify(target, result.Hops)
	if result.Error != "" {
		reason = result.Error
	}
	entryIP := ""
	for _, hop := range result.Hops {
		if hop.IP != "" {
			entryIP = hop.IP
		}
	}
	hopsJSON, err := json.Marshal(result.Hops)
	if err != nil {
		return err
	}
	db := dbcore.GetDBInstance()
	sample := models.ReturnRouteSample{ClientID: clientID, Carrier: target.Carrier, TargetID: target.ID, TargetHost: target.Host, RouteType: routeType, Confidence: confidence, EntryIP: entryIP, Reason: reason, HopsJSON: string(hopsJSON), OK: result.OK, TestedAt: now}
	if err := db.Create(&sample).Error; err != nil {
		return err
	}
	return aggregateClientCarrier(db, clientID, target.Carrier, now)
}

func aggregateClientCarrier(db *gorm.DB, clientID, carrier string, now time.Time) error {
	var samples []models.ReturnRouteSample
	if err := db.Where("client_id = ? AND carrier = ? AND tested_at >= ? AND ok = ?", clientID, carrier, now.Add(-48*time.Hour), true).Order("tested_at desc").Limit(12).Find(&samples).Error; err != nil {
		return err
	}
	if len(samples) == 0 {
		return touchSummary(db, clientID, carrier, now, false, "Unknown", "low", "no successful samples")
	}
	counts := make(map[string]int)
	var latest models.ReturnRouteSample
	for _, sample := range samples {
		if latest.ID == 0 {
			latest = sample
		}
		if sample.RouteType != "Unknown" {
			counts[sample.RouteType]++
		}
	}
	bestType, bestCount := "Unknown", 0
	for routeType, count := range counts {
		if count > bestCount {
			bestType, bestCount = routeType, count
		}
	}
	confidence := "low"
	reason := "insufficient stable path evidence"
	if bestCount >= 2 {
		confidence, reason = "high", fmt.Sprintf("%d recent targets agree", bestCount)
	} else if bestCount == 1 {
		confidence, reason = "medium", "one recent target produced a stable signature"
	}
	if len(counts) > 1 && bestCount < len(samples) {
		bestType = "Mixed"
		confidence, reason = "low", "recent targets produced different route signatures"
	}
	return touchSummary(db, clientID, carrier, now, true, bestType, confidence, reason)
}

func touchSummary(db *gorm.DB, clientID, carrier string, now time.Time, success bool, routeType, confidence, reason string) error {
	var row models.ReturnRouteResult
	err := db.Where("client_id = ? AND carrier = ?", clientID, carrier).First(&row).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == gorm.ErrRecordNotFound {
		row = models.ReturnRouteResult{ClientID: clientID, Carrier: carrier}
	}
	row.LastAttempt = now
	if success {
		row.RouteType, row.Confidence, row.Reason, row.TestedAt, row.Stale = routeType, confidence, reason, now, false
	} else if row.TestedAt.IsZero() {
		row.RouteType, row.Confidence, row.Reason, row.TestedAt, row.Stale = "Unknown", "low", reason, now, true
	} else {
		row.Stale = time.Since(row.TestedAt) > 48*time.Hour
	}
	return db.Save(&row).Error
}

func Summaries(clientID string) ([]models.ReturnRouteSummary, error) {
	var rows []models.ReturnRouteResult
	if err := dbcore.GetDBInstance().Where("client_id = ?", clientID).Order("carrier asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]models.ReturnRouteSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, models.ReturnRouteSummary{Carrier: row.Carrier, RouteType: row.RouteType, Confidence: row.Confidence, TestedAt: row.TestedAt, Stale: row.Stale || (!row.TestedAt.IsZero() && time.Since(row.TestedAt) > 48*time.Hour)})
	}
	return result, nil
}

func RunForClient(clientID string) int {
	if !agent_runtime.HasV2Capability(clientID, "trace:v1") || !agent_runtime.IsAgentOnline(clientID) {
		return 0
	}
	count := 0
	for _, target := range Targets() {
		if !target.Enabled {
			continue
		}
		params := v2.NextTraceParams{TaskID: "return-route-" + uuid.NewString(), SourceID: clientID, TargetID: target.ID, TargetHost: target.Host, IPFamily: target.IPFamily, Protocol: target.Protocol, MaxHops: 30, TimeoutMs: 20000}
		if agent_runtime.DispatchV2Event(clientID, v2.MethodNetworkTestNextTrace, params) {
			count++
		}
	}
	return count
}

func RunScheduled() {
	scheduleMu.Lock()
	defer scheduleMu.Unlock()
	all, err := clients.GetAllClientBasicInfo()
	if err != nil {
		return
	}
	started := 0
	for _, client := range all {
		if started >= 5 {
			break
		}
		if RunForClient(client.UUID) > 0 {
			started++
		}
	}
}

func CleanupSamples() error {
	return dbcore.GetDBInstance().Where("tested_at < ?", time.Now().UTC().Add(-7*24*time.Hour)).Delete(&models.ReturnRouteSample{}).Error
}
