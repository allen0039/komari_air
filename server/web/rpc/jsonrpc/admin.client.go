package jsonrpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/records"
	"github.com/komari-monitor/komari/database/returnroutes"
	"github.com/komari-monitor/komari/internal/agentdist"
	"github.com/komari-monitor/komari/internal/metricstore"
	"github.com/komari-monitor/komari/pkg/rpc"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

var agentUpgradeQueueMu sync.Mutex
var agentUpgradeQueueRunning bool

const agentUpgradeConfirmationTimeout = 3 * time.Minute

func adminGetAgentUpgradeStatus(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	return agent_runtime.GetUpgradeStatusSnapshot(), nil
}

func adminForceUpdateAgents(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		UUIDs []string `json:"uuids"`
	}
	// Empty UUIDs retains the existing all-node behavior; the UI sends selected
	// UUIDs for a safe one-node canary rollout.
	// Bind errors are treated as invalid input rather than silently upgrading all.
	if req != nil {
		if err := req.BindParams(&params); err != nil {
			return nil, rpc.MakeError(rpc.InvalidParams, "invalid uuids", nil)
		}
	}
	all, err := clients.GetAllClientBasicInfo()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	if len(params.UUIDs) > 0 {
		selected := make(map[string]bool, len(params.UUIDs))
		for _, uuid := range params.UUIDs {
			selected[uuid] = true
		}
		filtered := all[:0]
		for _, client := range all {
			if selected[client.UUID] {
				filtered = append(filtered, client)
			}
		}
		all = filtered
		if len(all) == 0 {
			return nil, rpc.MakeError(rpc.InvalidParams, "no matching clients", nil)
		}
	}
	targetVersion, err := agentdist.VersionForIP(all[0].IPv4)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "panel-managed agent build is unavailable", err.Error())
	}
	agentUpgradeQueueMu.Lock()
	if agentUpgradeQueueRunning {
		agentUpgradeQueueMu.Unlock()
		return nil, rpc.MakeError(rpc.InvalidParams, "agent upgrade queue is already running", nil)
	}
	agentUpgradeQueueRunning = true
	agentUpgradeQueueMu.Unlock()
	items := make([]agent_runtime.UpgradeStatus, 0, len(all))
	for _, client := range all {
		items = append(items, agent_runtime.UpgradeStatus{
			UUID:           client.UUID,
			Name:           client.Name,
			CurrentVersion: client.Version,
		})
	}
	agent_runtime.ResetUpgradeStatuses(targetVersion, items)
	go func() {
		defer func() {
			agent_runtime.FinishUpgradeRun()
			agentUpgradeQueueMu.Lock()
			agentUpgradeQueueRunning = false
			agentUpgradeQueueMu.Unlock()
		}()
		var confirmations sync.WaitGroup
		for i := 0; i < len(all); i += 2 {
			end := i + 2
			if end > len(all) {
				end = len(all)
			}
			for _, client := range all[i:end] {
				clientTargetVersion, versionErr := agentdist.VersionForIP(client.IPv4)
				if versionErr != nil {
					agent_runtime.SetUpgradeStatus(client.UUID, agent_runtime.UpgradeStateFailed, versionErr.Error(), client.Version)
					continue
				}
				if clientTargetVersion != targetVersion && len(params.UUIDs) > 0 {
					targetVersion = clientTargetVersion
				}
				if client.Version == clientTargetVersion {
					agent_runtime.SetUpgradeStatus(client.UUID, agent_runtime.UpgradeStateSucceeded, "已是目标版本", client.Version)
					continue
				}
				ok := agent_runtime.HasV2Capability(client.UUID, "config:v1") && agent_runtime.DispatchV2Event(client.UUID, v2.MethodAgentUpdate, nil)
				if !ok {
					agent_runtime.SetUpgradeStatus(client.UUID, agent_runtime.UpgradeStateFailed, "节点离线或当前 Agent 不支持远程升级", client.Version)
					continue
				}
				agent_runtime.SetUpgradeStatus(client.UUID, agent_runtime.UpgradeStateWaiting, "升级指令已发送，等待 Agent 返回结果", client.Version)
				confirmations.Add(1)
				go func(uuid string) {
					defer confirmations.Done()
					waitForAgentUpgrade(uuid, targetVersion)
				}(client.UUID)
			}
			if end < len(all) {
				time.Sleep(15 * time.Second)
			}
		}
		confirmations.Wait()
	}()
	return map[string]any{"queued": len(all), "batch_size": 2, "interval_seconds": 15, "target_version": targetVersion, "timeout_seconds": int(agentUpgradeConfirmationTimeout.Seconds())}, nil
}

func waitForAgentUpgrade(uuid, targetVersion string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timer := time.NewTimer(agentUpgradeConfirmationTimeout)
	defer timer.Stop()
	for {
		if agent_runtime.UpgradeStatusIsTerminal(uuid) {
			return
		}
		client, err := clients.GetClientByUUID(uuid)
		if err == nil && client.Version == targetVersion {
			agent_runtime.SetUpgradeStatus(uuid, agent_runtime.UpgradeStateSucceeded, "Agent 已重新上线并上报目标版本", client.Version)
			return
		}
		select {
		case <-ticker.C:
			continue
		case <-timer.C:
			currentVersion := ""
			if err == nil {
				currentVersion = client.Version
			}
			message := fmt.Sprintf("等待 %s 版本重新上线超时", targetVersion)
			if currentVersion != "" {
				message += fmt.Sprintf("，当前仍上报 %s", currentVersion)
			}
			agent_runtime.SetUpgradeStatus(uuid, agent_runtime.UpgradeStateTimeout, message, currentVersion)
			return
		}
	}
}

// admin.client.go
// client 资源的 RPC2 方法（admin 命名空间）。承载原 web/api/admin/client.go 的业务逻辑，
// 包含审计日志与运行时副作用。传统 REST handler 经 CallFromGin 转调这些方法。

func init() {
	RegisterWithGroupAndMeta("forceUpdateAgents", rpc.RoleAdmin, adminForceUpdateAgents, &rpc.MethodMeta{Name: "admin:forceUpdateAgents", Summary: "Force online agents to check for updates", Returns: "{ accepted: number }"})
	RegisterWithGroupAndMeta("getAgentUpgradeStatus", rpc.RoleAdmin, adminGetAgentUpgradeStatus, &rpc.MethodMeta{Name: "admin:getAgentUpgradeStatus", Summary: "Get agent upgrade status", Returns: "object"})
	RegisterWithGroupAndMeta("addClient", rpc.RoleAdmin, adminAddClient, &rpc.MethodMeta{
		Name:    "admin:addClient",
		Summary: "Create a new client",
		Params: []rpc.ParamMeta{
			{Name: "name", Type: "string", Required: false, Description: "Optional client name"},
		},
		Returns: "{ uuid: string, token: string }",
	})
	RegisterWithGroupAndMeta("editClient", rpc.RoleAdmin, adminEditClient, &rpc.MethodMeta{
		Name:    "admin:editClient",
		Summary: "Edit a client (partial update)",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
		},
		Returns: "null",
	})
	RegisterWithGroupAndMeta("removeClient", rpc.RoleAdmin, adminRemoveClient, &rpc.MethodMeta{
		Name:    "admin:removeClient",
		Summary: "Delete a client",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
		},
		Returns: "null",
	})
	RegisterWithGroupAndMeta("getClient", rpc.RoleAdmin, adminGetClient, &rpc.MethodMeta{
		Name:    "admin:getClient",
		Summary: "Get a client by UUID",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
		},
		Returns: "Client",
	})
	RegisterWithGroupAndMeta("listClients", rpc.RoleAdmin, adminListClients, &rpc.MethodMeta{
		Name:    "admin:listClients",
		Summary: "List all clients (basic info)",
		Returns: "Client[]",
	})
	RegisterWithGroupAndMeta("getClientToken", rpc.RoleAdmin, adminGetClientToken, &rpc.MethodMeta{
		Name:    "admin:getClientToken",
		Summary: "Get a client's token by UUID",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
		},
		Returns: "{ token: string }",
	})
	RegisterWithGroupAndMeta("clearRecords", rpc.RoleAdmin, adminClearRecords, &rpc.MethodMeta{
		Name:    "admin:clearRecords",
		Summary: "Delete all load records",
		Returns: "null",
	})
}

// auditActor 从上下文提取审计用的 actor UUID 与来源 IP。
func auditActor(ctx context.Context) (uuid, ip string) {
	if meta := rpc.MetaFromContext(ctx); meta != nil {
		uuid = meta.UserUUID
		ip = meta.RemoteIP
	}
	return uuid, ip
}

func adminAddClient(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		Name string `json:"name"`
	}
	req.BindParams(&params)

	var (
		uuid, token string
		err         error
	)
	if params.Name == "" {
		uuid, token, err = clients.CreateClient()
	} else {
		uuid, token, err = clients.CreateClientWithName(params.Name)
	}
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	if params.Name != "" {
		actor, ip := auditActor(ctx)
		auditlog.Log(ip, actor, "create client:"+uuid, "info")
	}
	return map[string]any{"uuid": uuid, "token": token}, nil
}

func adminEditClient(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var update map[string]interface{}
	if err := req.BindParams(&update); err != nil || update == nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid params", nil)
	}
	uuid, _ := update["uuid"].(string)
	if uuid == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid or missing UUID", nil)
	}
	if err := clients.SaveClient(update); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "edit client:"+uuid, "info")
	return nil, nil
}

func adminRemoveClient(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		UUID string `json:"uuid"`
	}
	req.BindParams(&params)
	if params.UUID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid or missing UUID", nil)
	}
	if err := clients.DeleteClient(params.UUID); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to delete client"+err.Error(), nil)
	}
	metricstore.DeleteEntityAsync(params.UUID)
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "delete client:"+params.UUID, "warn")
	agent_runtime.DeleteConnectedClients(params.UUID)
	agent_runtime.DeleteLatestReport(params.UUID)
	return nil, nil
}

func adminGetClient(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		UUID string `json:"uuid"`
	}
	req.BindParams(&params)
	if params.UUID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid or missing UUID", nil)
	}
	result, err := clients.GetClientByUUID(params.UUID)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	return result, nil
}

func adminListClients(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	cls, err := clients.GetAllClientBasicInfo()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	for i := range cls {
		if routes, routeErr := returnroutes.Summaries(cls[i].UUID); routeErr == nil {
			cls[i].ReturnRoutes = routes
		}
	}
	return cls, nil
}

func adminGetClientToken(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		UUID string `json:"uuid"`
	}
	req.BindParams(&params)
	if params.UUID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid or missing UUID", nil)
	}
	token, err := clients.GetClientTokenByUUID(params.UUID)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	return map[string]any{"token": token}, nil
}

func adminClearRecords(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	if err := records.DeleteAll(); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to delete Record"+err.Error(), nil)
	}
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "clear records", "warn")
	return nil, nil
}
