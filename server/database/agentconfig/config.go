package agentconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	"gorm.io/gorm"
)

func Validate(config v2.AgentManagedConfig) error {
	if config.Interval < 1 || config.Interval > 3600 {
		return fmt.Errorf("interval must be between 1 and 3600 seconds")
	}
	if config.MonthRotate < 0 || config.MonthRotate > 31 {
		return fmt.Errorf("month_rotate must be between 0 and 31")
	}
	if len(config.IncludeNics) > 2048 || len(config.ExcludeNics) > 2048 || len(config.IncludeMountpoints) > 4096 {
		return fmt.Errorf("managed config text field is too long")
	}
	return nil
}

func Get(uuid string) (models.AgentConfig, error) {
	var row models.AgentConfig
	err := dbcore.GetDBInstance().Where("uuid = ?", uuid).First(&row).Error
	return row, err
}

func GetOrCreate(uuid string) (models.AgentConfig, error) {
	row, err := Get(uuid)
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.AgentConfig{}, err
	}
	now := time.Now().UTC()
	row = models.AgentConfig{UUID: uuid, SyncStatus: "unknown", CreatedAt: now, UpdatedAt: now}
	if err := dbcore.GetDBInstance().Create(&row).Error; err != nil {
		return models.AgentConfig{}, err
	}
	return row, nil
}

func UpdateDesired(uuid string, config v2.AgentManagedConfig) (models.AgentConfig, error) {
	if err := Validate(config); err != nil {
		return models.AgentConfig{}, err
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return models.AgentConfig{}, err
	}
	row, err := GetOrCreate(uuid)
	if err != nil {
		return models.AgentConfig{}, err
	}
	baseRevision := row.DesiredRevision
	if row.ReportedRevision > baseRevision {
		baseRevision = row.ReportedRevision
	}
	next := baseRevision + 1
	if next == 0 {
		next = 1
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"desired_config":   string(raw),
		"desired_revision": next,
		"sync_status":      "pending",
		"last_error":       "",
		"updated_at":       now,
	}
	if err := dbcore.GetDBInstance().Model(&models.AgentConfig{}).Where("uuid = ?", uuid).Updates(updates).Error; err != nil {
		return models.AgentConfig{}, err
	}
	return Get(uuid)
}

func UpdateReported(uuid string, report v2.ConfigReportParams) (models.AgentConfig, error) {
	if err := Validate(report.Config); err != nil && report.Status != "error" {
		return models.AgentConfig{}, err
	}
	raw, err := json.Marshal(report.Config)
	if err != nil {
		return models.AgentConfig{}, err
	}
	row, err := GetOrCreate(uuid)
	if err != nil {
		return models.AgentConfig{}, err
	}
	now := time.Now().UTC()
	status := report.Status
	if status == "" {
		status = "applied"
	}
	syncStatus := "pending"
	lastSynced := row.LastSyncedAt
	if status == "error" {
		syncStatus = "error"
	} else if row.DesiredRevision == 0 || report.Revision >= row.DesiredRevision {
		syncStatus = "synced"
		lastSynced = &now
	}
	updates := map[string]any{
		"sync_status":    syncStatus,
		"last_error":     report.Error,
		"last_synced_at": lastSynced,
		"updated_at":     now,
	}
	if status != "error" {
		updates["reported_config"] = string(raw)
		updates["reported_revision"] = report.Revision
	}
	if err := dbcore.GetDBInstance().Model(&models.AgentConfig{}).Where("uuid = ?", uuid).Updates(updates).Error; err != nil {
		return models.AgentConfig{}, err
	}
	return Get(uuid)
}

func Desired(row models.AgentConfig) (v2.AgentManagedConfig, bool) {
	var config v2.AgentManagedConfig
	if row.DesiredConfig == "" {
		return config, false
	}
	if json.Unmarshal([]byte(row.DesiredConfig), &config) != nil {
		return v2.AgentManagedConfig{}, false
	}
	return config, true
}

func Reported(row models.AgentConfig) (v2.AgentManagedConfig, bool) {
	var config v2.AgentManagedConfig
	if row.ReportedConfig == "" {
		return config, false
	}
	if json.Unmarshal([]byte(row.ReportedConfig), &config) != nil {
		return v2.AgentManagedConfig{}, false
	}
	return config, true
}
