package jsonrpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/komari-monitor/komari/database/returnroutes"
	"github.com/komari-monitor/komari/pkg/rpc"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

func init() {
	RegisterWithGroupAndMeta("listReturnRouteTargets", rpc.RoleAdmin, adminListReturnRouteTargets, &rpc.MethodMeta{Name: "admin:listReturnRouteTargets", Summary: "List configured return route targets", Returns: "ReturnRouteTarget[]"})
	RegisterWithGroupAndMeta("saveReturnRouteTargets", rpc.RoleAdmin, adminSaveReturnRouteTargets, &rpc.MethodMeta{Name: "admin:saveReturnRouteTargets", Summary: "Save three return route targets", Params: []rpc.ParamMeta{{Name: "targets", Type: "ReturnRouteTarget[]", Required: true}}, Returns: "ReturnRouteTarget[]"})
	RegisterWithGroupAndMeta("runReturnRoutes", rpc.RoleAdmin, adminRunReturnRoutes, &rpc.MethodMeta{Name: "admin:runReturnRoutes", Summary: "Run all configured return route targets on a client", Params: []rpc.ParamMeta{{Name: "uuid", Type: "string", Required: true}}, Returns: "{ accepted: number }"})
	RegisterWithGroupAndMeta("runReturnRoute", rpc.RoleAdmin, adminRunReturnRoute, &rpc.MethodMeta{Name: "admin:runReturnRoute", Summary: "Run a return route trace on a client", Params: []rpc.ParamMeta{{Name: "uuid", Type: "string", Required: true}, {Name: "target_id", Type: "string", Required: true}, {Name: "target_host", Type: "string", Required: true}}, Returns: "{ accepted: boolean, task_id: string }"})
	RegisterWithGroupAndMeta("getReturnRoutes", rpc.RoleAdmin, adminGetReturnRoutes, &rpc.MethodMeta{Name: "admin:getReturnRoutes", Summary: "Get return route summaries", Params: []rpc.ParamMeta{{Name: "uuid", Type: "string", Required: true}}, Returns: "ReturnRouteSummary[]"})
	RegisterWithGroupAndMeta("listReturnRouteLogs", rpc.RoleAdmin, adminListReturnRouteLogs, &rpc.MethodMeta{Name: "admin:listReturnRouteLogs", Summary: "List recent return route probe logs", Returns: "{ logs: ReturnRouteLog[], total: number }"})
	RegisterWithGroupAndMeta("runAllReturnRoutes", rpc.RoleAdmin, adminRunAllReturnRoutes, &rpc.MethodMeta{Name: "admin:runAllReturnRoutes", Summary: "Run return route probes on all online agents", Returns: "{ accepted: number }"})
	RegisterWithGroupAndMeta("clearReturnRouteLogs", rpc.RoleAdmin, adminClearReturnRouteLogs, &rpc.MethodMeta{Name: "admin:clearReturnRouteLogs", Summary: "Clear return route probe logs and summaries", Params: []rpc.ParamMeta{{Name: "confirm", Type: "boolean", Required: true}}, Returns: "{ cleared: boolean }"})
}

func adminListReturnRouteLogs(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if err := req.BindParams(&p); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, err.Error(), nil)
	}
	logs, total, err := returnroutes.ListLogs(p.Limit, p.Offset)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	return map[string]any{"logs": logs, "total": total}, nil
}

func adminRunAllReturnRoutes(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	accepted, err := returnroutes.RunAll()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	return map[string]any{"accepted": accepted}, nil
}

func adminClearReturnRouteLogs(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		Confirm bool `json:"confirm"`
	}
	if err := req.BindParams(&p); err != nil || !p.Confirm {
		return nil, rpc.MakeError(rpc.InvalidParams, "confirm must be true", nil)
	}
	if err := returnroutes.ClearLogs(); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	return map[string]any{"cleared": true}, nil
}

func adminListReturnRouteTargets(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	return returnroutes.Targets(), nil
}

func adminSaveReturnRouteTargets(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		Targets []returnroutes.Target `json:"targets"`
	}
	if err := req.BindParams(&p); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "targets are required", nil)
	}
	targets, err := returnroutes.SaveTargets(p.Targets)
	if err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, err.Error(), nil)
	}
	return targets, nil
}

func adminRunReturnRoutes(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		UUID string `json:"uuid"`
	}
	if err := req.BindParams(&p); err != nil || p.UUID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "uuid is required", nil)
	}
	if !agent_runtime.HasV2Capability(p.UUID, "trace:v1") {
		return nil, rpc.MakeError(rpc.InvalidParams, "agent does not support trace:v1", nil)
	}
	accepted := returnroutes.RunForClient(p.UUID)
	if accepted == 0 {
		return nil, rpc.MakeError(rpc.InvalidParams, "agent is offline or no targets are enabled", nil)
	}
	return map[string]any{"accepted": accepted}, nil
}

func adminRunReturnRoute(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		UUID       string `json:"uuid"`
		TargetID   string `json:"target_id"`
		TargetHost string `json:"target_host"`
	}
	if err := req.BindParams(&p); err != nil || p.UUID == "" || p.TargetID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "uuid and target_id are required", nil)
	}
	target, ok := returnroutes.TargetByID(p.TargetID)
	if !ok {
		return nil, rpc.MakeError(rpc.InvalidParams, "unknown return route target", nil)
	}
	if p.TargetHost == "" {
		p.TargetHost = target.Host
	}
	if !safeTraceTarget(p.TargetHost) {
		return nil, rpc.MakeError(rpc.InvalidParams, "target_host must resolve to a public IPv4 address", nil)
	}
	if !agent_runtime.HasV2Capability(p.UUID, "trace:v1") {
		return nil, rpc.MakeError(rpc.InvalidParams, "agent does not support trace:v1", nil)
	}
	taskID := fmt.Sprintf("return-route-%d", time.Now().UnixNano())
	ok = agent_runtime.DispatchV2Event(p.UUID, v2.MethodNetworkTestNextTrace, v2.NextTraceParams{TaskID: taskID, SourceID: p.UUID, TargetID: p.TargetID, TargetHost: p.TargetHost, IPFamily: v2.IPFamilyIPv4, Protocol: v2.TraceProtocolICMP, MaxHops: 30, TimeoutMs: 20000})
	if !ok {
		return nil, rpc.MakeError(rpc.InvalidParams, "agent is offline", nil)
	}
	return map[string]any{"accepted": true, "task_id": taskID}, nil
}

func safeTraceTarget(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return ip.To4() != nil && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsMulticast()
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		return false
	}
	for _, ip := range addrs {
		if ip.To4() != nil && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsMulticast() {
			return true
		}
	}
	return false
}

func adminGetReturnRoutes(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		UUID string `json:"uuid"`
	}
	if err := req.BindParams(&p); err != nil || p.UUID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "uuid is required", nil)
	}
	rows, err := returnroutes.Summaries(p.UUID)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	return rows, nil
}
