package returnroutes

import (
	"encoding/json"
	"fmt"
	"net"
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
var resultMu sync.Mutex
var queueMu sync.Mutex
var queueRunning bool

func classify(target Target, hops []v2.TraceHop) (string, string, string) {
	_ = target // The route is classified from the path, not from the destination carrier.
	path := asPath(hops)
	pathText := strings.Join(path, " → ")
	if pathText == "" {
		return "Unknown", "low", "未识别到有效 AS 路径"
	}
	has := func(asn string) bool {
		for _, item := range path {
			if item == asn {
				return true
			}
		}
		return false
	}
	// MiaoMiaoWu X prioritizes premium transit networks anywhere in the path.
	// CTG GIA is its special name for DMIT -> CTG (AS23764) -> CN2 (AS4809).
	if has("4809") {
		if has("23764") {
			return "CTG GIA", "high", "AS 路径 " + pathText
		}
		return "CN2 GIA", "high", "AS 路径 " + pathText
	}
	if has("9929") {
		return "9929", "high", "AS 路径 " + pathText
	}
	if has("10099") {
		return "10099", "high", "AS 路径 " + pathText
	}
	if has("58807") {
		return "CMIN2", "high", "AS 路径 " + pathText
	}
	// For ordinary backbones, the first carrier backbone in the observed path
	// wins. This reproduces e.g. AS4134 -> AS4837 being shown as 163.
	for _, asn := range path {
		switch asn {
		case "4134", "58466":
			return "163", "high", "AS 路径 " + pathText
		case "4837":
			return "4837", "high", "AS 路径 " + pathText
		case "56040", "56041", "58453", "9808":
			return "CMI", "high", "AS 路径 " + pathText
		}
	}
	return "Unknown", "low", "回程 AS 路径未命中已知骨干: " + pathText
}

func asPath(hops []v2.TraceHop) []string {
	path := make([]string, 0, len(hops))
	for _, hop := range hops {
		asn := normalizedASN(hop)
		if asn == "" || (len(path) > 0 && path[len(path)-1] == asn) {
			continue
		}
		path = append(path, asn)
	}
	return path
}

func normalizedASN(hop v2.TraceHop) string {
	asn := strings.TrimSpace(strings.ToUpper(hop.ASN))
	asn = strings.TrimPrefix(asn, "AS")
	if asn != "" {
		return asn
	}
	ip := net.ParseIP(strings.TrimSpace(hop.IP))
	if ip == nil || ip.To4() == nil {
		return ""
	}
	host := strings.ToLower(hop.Host)
	// NextTrace can identify CUII even when its numeric ASN is absent.
	// Match a complete metadata token; generic China Unicom/CNC labels are
	// insufficient, and an explicit ASN above always takes precedence.
	for _, field := range strings.Fields(host) {
		if strings.Trim(field, "[](),;") == "cuii" {
			return "9929"
		}
	}
	switch {
	case hasPrefix(ip, "59.43."), strings.Contains(host, "cn2-backbone"), strings.Contains(host, "ctcn2"):
		return "4809"
	case hasPrefix(ip, "218.105."), hasPrefix(ip, "210.51."):
		return "9929"
	case hasPrefix(ip, "162.219.85."), hasPrefix(ip, "162.255.48."), hasPrefix(ip, "203.160.75."):
		return "10099"
	case hasPrefix(ip, "202.97."), strings.Contains(host, "ct163"):
		return "4134"
	case hasPrefix(ip, "219.158."):
		return "4837"
	case hasPrefix(ip, "223.120.196."), hasPrefix(ip, "223.120.197."), hasPrefix(ip, "223.120.200."):
		return "58807"
	case hasPrefix(ip, "223.120."):
		return "58453"
	case hasPrefix(ip, "221.183."), hasPrefix(ip, "221.176."), hasPrefix(ip, "111.24."):
		return "9808"
	}
	return ""
}

func routeEntry(routeType string, hops []v2.TraceHop) (string, string) {
	preferred := map[string][]string{
		"CTG GIA": {"23764", "4809"},
		"CN2 GIA": {"4809"},
		"9929":    {"10099", "9929"},
		"10099":   {"10099"},
		"CMIN2":   {"58807"},
		"CMI":     {"56040", "56041", "58453", "9808"},
		"163":     {"4134", "58466"},
		"4837":    {"4837"},
	}
	for _, wanted := range preferred[routeType] {
		for _, hop := range hops {
			if normalizedASN(hop) == wanted && hop.IP != "" {
				return hop.IP, wanted
			}
		}
	}
	return "", ""
}

func SaveResult(clientID string, result v2.NextTraceResult) error {
	resultMu.Lock()
	defer resultMu.Unlock()
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
	entryIP, entryASN := routeEntry(routeType, result.Hops)
	if entryIP != "" {
		reason = fmt.Sprintf("回国跳点 %s (AS%s)，%s", entryIP, entryASN, reason)
	}
	hopsJSON, err := json.Marshal(result.Hops)
	if err != nil {
		return err
	}
	db := dbcore.GetDBInstance()
	sample := models.ReturnRouteSample{TaskID: result.TaskID, ClientID: clientID, Carrier: target.Carrier, TargetID: target.ID, TargetHost: target.Host, RouteType: routeType, Confidence: confidence, EntryIP: entryIP, EntryASN: entryASN, Reason: reason, HopsJSON: string(hopsJSON), OK: result.OK, TestedAt: now}
	var retry bool
	err = db.Transaction(func(tx *gorm.DB) error {
		// Retries reuse the original timestamp; a lost acknowledgement must
		// not turn one observation into two confirmations.
		var count int64
		if err := tx.Model(&models.ReturnRouteSample{}).Where("client_id = ? AND task_id = ? AND task_id <> ''", clientID, result.TaskID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		if err := tx.Create(&sample).Error; err != nil {
			return err
		}
		if err := aggregateClientCarrier(tx, clientID, target.Carrier, now); err != nil {
			return err
		}
		var row models.ReturnRouteResult
		if err := tx.Where("client_id = ? AND carrier = ?", clientID, target.Carrier).First(&row).Error; err != nil {
			return err
		}
		retry = row.Stale
		return nil
	})
	if err == nil && retry && !strings.HasSuffix(result.TaskID, ":retry") {
		agent_runtime.DispatchV2Event(clientID, v2.MethodNetworkTestNextTrace, v2.NextTraceParams{
			TaskID: result.TaskID + ":retry", SourceID: clientID, TargetID: target.ID,
			TargetHost: target.Host, IPFamily: target.IPFamily, Protocol: target.Protocol,
			MaxHops: 30, TimeoutMs: 20000,
		})
	}
	return err
}

func aggregateClientCarrier(db *gorm.DB, clientID, carrier string, now time.Time) error {
	var samples []models.ReturnRouteSample
	if err := db.Where("client_id = ? AND carrier = ? AND archived = ?", clientID, carrier, false).Order("tested_at desc, id desc").Limit(2).Find(&samples).Error; err != nil {
		return err
	}
	var row models.ReturnRouteResult
	err := db.Where("client_id = ? AND carrier = ?", clientID, carrier).First(&row).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if len(samples) == 0 {
		return touchSummary(db, clientID, carrier, now, nil)
	}
	latest := &samples[0]
	if latest.OK && latest.RouteType != "Unknown" && latest.Confidence == "high" {
		unchanged := row.RouteType == "" || row.RouteType == "Unknown" || row.RouteType == latest.RouteType
		confirmed := len(samples) > 1 && samples[1].OK && samples[1].Confidence == "high" && samples[1].RouteType == latest.RouteType
		if unchanged || confirmed {
			return touchSummary(db, clientID, carrier, now, latest)
		}
	}
	return touchSummary(db, clientID, carrier, now, nil)
}

func touchSummary(db *gorm.DB, clientID, carrier string, now time.Time, sample *models.ReturnRouteSample) error {
	var row models.ReturnRouteResult
	err := db.Where("client_id = ? AND carrier = ?", clientID, carrier).First(&row).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == gorm.ErrRecordNotFound {
		row = models.ReturnRouteResult{ClientID: clientID, Carrier: carrier}
	}
	if row.LastAttempt.After(now) {
		now = row.LastAttempt
	}
	row.LastAttempt = now
	if sample != nil {
		row.RouteType = sample.RouteType
		row.Confidence = sample.Confidence
		row.TargetID = sample.TargetID
		row.EntryIP = sample.EntryIP
		row.EntryASN = sample.EntryASN
		row.Reason = sample.Reason
		row.TestedAt = sample.TestedAt
		row.Stale = false
	} else if row.TestedAt.IsZero() {
		row.RouteType, row.Confidence, row.Reason, row.TestedAt, row.Stale = "Unknown", "low", "没有成功的探测结果", now, true
	} else {
		row.Stale = true
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
	if !ProbeEnabled() {
		return 0
	}
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
	_, _ = RunAllQueued()
}

// RunAll dispatches enabled targets to every online capable agent.
func RunAll() (int, error) {
	all, err := clients.GetAllClientBasicInfo()
	if err != nil {
		return 0, err
	}
	accepted := 0
	for _, client := range all {
		accepted += RunForClient(client.UUID)
	}
	return accepted, nil
}

// RunAllQueued probes one server at a time. Each server receives its three
// target tasks, then the queue waits beyond the agent timeout before moving on.
func RunAllQueued() (int, error) {
	all, err := clients.GetAllClientBasicInfo()
	if err != nil {
		return 0, err
	}
	queueMu.Lock()
	if queueRunning {
		queueMu.Unlock()
		return 0, fmt.Errorf("return route probe queue is already running")
	}
	queueRunning = true
	queueMu.Unlock()
	go func() {
		defer func() { queueMu.Lock(); queueRunning = false; queueMu.Unlock() }()
		for _, client := range all {
			RunForClient(client.UUID)
			time.Sleep(25 * time.Second)
		}
	}()
	return len(all), nil
}

type ProbeLog struct {
	ID         uint      `json:"id"`
	TaskID     string    `json:"task_id"`
	ClientID   string    `json:"client_id"`
	ClientName string    `json:"client_name"`
	Carrier    string    `json:"carrier"`
	TargetHost string    `json:"target_host"`
	RouteType  string    `json:"route_type"`
	Confidence string    `json:"confidence"`
	Reason     string    `json:"reason"`
	OK         bool      `json:"ok"`
	TestedAt   time.Time `json:"tested_at"`
}

type SettingsInfo struct {
	ScheduleTime  string `json:"schedule_time"`
	StorageBytes  int64  `json:"storage_bytes"`
	LogCount      int64  `json:"log_count"`
	RetentionDays int    `json:"retention_days"`
}

func Settings() (SettingsInfo, error) {
	db := dbcore.GetDBInstance()
	var count int64
	if err := db.Model(&models.ReturnRouteSample{}).Where("hidden = ?", false).Count(&count).Error; err != nil {
		return SettingsInfo{}, err
	}
	var bytes int64
	if err := db.Model(&models.ReturnRouteSample{}).Where("hidden = ?", false).Select("COALESCE(SUM(LENGTH(task_id)+LENGTH(target_host)+LENGTH(route_type)+LENGTH(reason)+LENGTH(hops_json)), 0)").Scan(&bytes).Error; err != nil {
		return SettingsInfo{}, err
	}
	return SettingsInfo{ScheduleTime: ScheduleTime(), StorageBytes: bytes, LogCount: count, RetentionDays: RetentionDays()}, nil
}

func ListLogs(limit, offset int) ([]ProbeLog, int64, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	db := dbcore.GetDBInstance()
	var total int64
	if err := db.Model(&models.ReturnRouteSample{}).Where("hidden = ?", false).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []ProbeLog
	err := db.Table("return_route_samples AS samples").
		Select("samples.id, samples.task_id, samples.client_id, clients.name AS client_name, samples.carrier, samples.target_host, samples.route_type, samples.confidence, samples.reason, samples.ok, samples.tested_at").
		Joins("LEFT JOIN clients ON clients.uuid = samples.client_id").
		Where("samples.hidden = ?", false).
		Order("samples.tested_at DESC, samples.id DESC").Limit(limit).Offset(offset).Scan(&logs).Error
	return logs, total, err
}

func ClearLogs() error {
	resultMu.Lock()
	defer resultMu.Unlock()
	return clearLogs(dbcore.GetDBInstance())
}

func clearLogs(db *gorm.DB) error {
	return db.Model(&models.ReturnRouteSample{}).Where("hidden = ?", false).Update("hidden", true).Error
}

// ClearResults removes visible labels while retaining the historical probe log.
// Existing samples are archived so a later probe cannot reuse pre-clear evidence.
func ClearResults() error {
	resultMu.Lock()
	defer resultMu.Unlock()
	return clearResults(dbcore.GetDBInstance())
}

func clearResults(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.ReturnRouteSample{}).Where("archived = ?", false).Update("archived", true).Error; err != nil {
			return err
		}
		return tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.ReturnRouteResult{}).Error
	})
}

func CleanupSamples() error {
	return dbcore.GetDBInstance().Where("tested_at < ?", time.Now().UTC().Add(-time.Duration(RetentionDays())*24*time.Hour)).Delete(&models.ReturnRouteSample{}).Error
}

func hasPrefix(ip net.IP, prefix string) bool {
	return strings.HasPrefix(ip.To4().String(), prefix)
}
