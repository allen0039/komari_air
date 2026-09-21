package jsonrpc

import (
	"context"

	"github.com/komari-monitor/komari/database/agentconfig"
	"github.com/komari-monitor/komari/pkg/rpc"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

func init() {
	RegisterWithGroupAndMeta("getAgentConfig", rpc.RoleAdmin, adminGetAgentConfig, &rpc.MethodMeta{
		Name: "admin:getAgentConfig", Summary: "Get desired/reported managed agent configuration",
	})
	RegisterWithGroupAndMeta("updateAgentConfig", rpc.RoleAdmin, adminUpdateAgentConfig, &rpc.MethodMeta{
		Name: "admin:updateAgentConfig", Summary: "Update desired managed agent configuration",
	})
	RegisterWithGroupAndMeta("retryAgentConfigSync", rpc.RoleAdmin, adminRetryAgentConfigSync, &rpc.MethodMeta{
		Name: "admin:retryAgentConfigSync", Summary: "Retry managed agent configuration sync",
	})
}

func agentConfigOutput(uuid string) (map[string]any, *rpc.JsonRpcError) {
	row, err := agentconfig.GetOrCreate(uuid)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	desired, hasDesired := agentconfig.Desired(row)
	reported, hasReported := agentconfig.Reported(row)
	return map[string]any{
		"uuid":              uuid,
		"desired":           desired,
		"reported":          reported,
		"has_desired":       hasDesired,
		"has_reported":      hasReported,
		"desired_revision":  row.DesiredRevision,
		"reported_revision": row.ReportedRevision,
		"status":            row.SyncStatus,
		"last_error":        row.LastError,
		"last_synced_at":    row.LastSyncedAt,
		"supported":         agent_runtime.HasV2Capability(uuid, "config:v1"),
		"online":            agent_runtime.IsAgentOnline(uuid),
	}, nil
}

func adminGetAgentConfig(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		UUID string `json:"uuid"`
	}
	if err := req.BindParams(&params); err != nil || params.UUID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "missing uuid", nil)
	}
	return agentConfigOutput(params.UUID)
}

func adminUpdateAgentConfig(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		UUID   string                `json:"uuid"`
		Config v2.AgentManagedConfig `json:"config"`
	}
	if err := req.BindParams(&params); err != nil || params.UUID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "invalid params", nil)
	}
	row, err := agentconfig.UpdateDesired(params.UUID, params.Config)
	if err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, err.Error(), nil)
	}
	if desired, ok := agentconfig.Desired(row); ok && agent_runtime.HasV2Capability(params.UUID, "config:v1") {
		agent_runtime.DispatchV2Event(params.UUID, v2.MethodAgentConfigSet, v2.ConfigSetParams{
			Revision: row.DesiredRevision,
			Config:   desired,
		})
	}
	return agentConfigOutput(params.UUID)
}

func adminRetryAgentConfigSync(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		UUID string `json:"uuid"`
	}
	if err := req.BindParams(&params); err != nil || params.UUID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "missing uuid", nil)
	}
	row, err := agentconfig.GetOrCreate(params.UUID)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	if desired, ok := agentconfig.Desired(row); ok && agent_runtime.HasV2Capability(params.UUID, "config:v1") {
		agent_runtime.DispatchV2Event(params.UUID, v2.MethodAgentConfigSet, v2.ConfigSetParams{
			Revision: row.DesiredRevision,
			Config:   desired,
		})
	}
	return agentConfigOutput(params.UUID)
}
