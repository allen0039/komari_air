package agent

import (
	"sync"
	"time"
)

const (
	UpgradeStateQueued     = "queued"
	UpgradeStateWaiting    = "waiting"
	UpgradeStateRestarting = "restarting"
	UpgradeStateSucceeded  = "succeeded"
	UpgradeStateFailed     = "failed"
	UpgradeStateTimeout    = "timeout"
)

type UpgradeStatus struct {
	UUID           string    `json:"uuid"`
	Name           string    `json:"name"`
	State          string    `json:"state"`
	Message        string    `json:"message,omitempty"`
	CurrentVersion string    `json:"current_version,omitempty"`
	TargetVersion  string    `json:"target_version"`
	StartedAt      time.Time `json:"started_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UpgradeStatusSnapshot struct {
	Running       bool                     `json:"running"`
	TargetVersion string                   `json:"target_version"`
	Items         map[string]UpgradeStatus `json:"items"`
}

var upgradeStatuses = struct {
	sync.RWMutex
	running       bool
	targetVersion string
	items         map[string]UpgradeStatus
}{items: make(map[string]UpgradeStatus)}

func ResetUpgradeStatuses(targetVersion string, items []UpgradeStatus) {
	now := time.Now().UTC()
	upgradeStatuses.Lock()
	defer upgradeStatuses.Unlock()
	upgradeStatuses.running = true
	upgradeStatuses.targetVersion = targetVersion
	upgradeStatuses.items = make(map[string]UpgradeStatus, len(items))
	for _, item := range items {
		item.State = UpgradeStateQueued
		item.TargetVersion = targetVersion
		item.StartedAt = now
		item.UpdatedAt = now
		upgradeStatuses.items[item.UUID] = item
	}
}

func FinishUpgradeRun() {
	upgradeStatuses.Lock()
	upgradeStatuses.running = false
	upgradeStatuses.Unlock()
}

func GetUpgradeStatusSnapshot() UpgradeStatusSnapshot {
	upgradeStatuses.RLock()
	defer upgradeStatuses.RUnlock()
	items := make(map[string]UpgradeStatus, len(upgradeStatuses.items))
	for uuid, item := range upgradeStatuses.items {
		items[uuid] = item
	}
	return UpgradeStatusSnapshot{
		Running:       upgradeStatuses.running,
		TargetVersion: upgradeStatuses.targetVersion,
		Items:         items,
	}
}

func SetUpgradeStatus(uuid, state, message, currentVersion string) bool {
	upgradeStatuses.Lock()
	defer upgradeStatuses.Unlock()
	item, ok := upgradeStatuses.items[uuid]
	if !ok || isTerminalUpgradeState(item.State) {
		return false
	}
	item.State = state
	item.Message = message
	if currentVersion != "" {
		item.CurrentVersion = currentVersion
	}
	item.UpdatedAt = time.Now().UTC()
	upgradeStatuses.items[uuid] = item
	return true
}

func UpgradeStatusIsTerminal(uuid string) bool {
	upgradeStatuses.RLock()
	defer upgradeStatuses.RUnlock()
	item, ok := upgradeStatuses.items[uuid]
	return !ok || isTerminalUpgradeState(item.State)
}

func ApplyAgentUpdateResult(uuid, status, version, errorMessage string) {
	upgradeStatuses.Lock()
	defer upgradeStatuses.Unlock()
	item, ok := upgradeStatuses.items[uuid]
	if !ok || isTerminalUpgradeState(item.State) {
		return
	}
	item.UpdatedAt = time.Now().UTC()
	if version != "" {
		item.CurrentVersion = version
	}
	switch status {
	case "checking":
		item.State = UpgradeStateWaiting
		item.Message = "Agent 已收到升级指令，正在检查更新"
	case "installed":
		item.State = UpgradeStateRestarting
		item.Message = "新版本已安装，等待 Agent 重启并重新上线"
	case "up_to_date":
		if version == item.TargetVersion {
			item.State = UpgradeStateSucceeded
			item.Message = "已是目标版本"
		} else {
			item.State = UpgradeStateFailed
			item.Message = "Agent 报告无需更新，但版本与面板目标版本不一致"
		}
	case "failed":
		item.State = UpgradeStateFailed
		item.Message = errorMessage
		if item.Message == "" {
			item.Message = "Agent 升级失败"
		}
	default:
		return
	}
	upgradeStatuses.items[uuid] = item
}

func isTerminalUpgradeState(state string) bool {
	return state == UpgradeStateSucceeded || state == UpgradeStateFailed || state == UpgradeStateTimeout
}
