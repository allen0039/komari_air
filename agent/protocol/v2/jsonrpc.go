package v2

import (
	"encoding/json"
	"time"
)

const (
	Version                 = "2.0"
	MethodAgentReport       = "agent.report"
	MethodAgentBasicInfo    = "agent.basicInfo"
	MethodAgentPingResult   = "agent.pingResult"
	MethodAgentPing         = "agent.ping"
	MethodAgentMessage      = "agent.message"
	MethodAgentEvent        = "agent.event"
	MethodAgentPull         = "agent.pull"
	MethodAgentConfigSet    = "agent.config.set"
	MethodAgentConfigReport = "agent.config.report"
	MethodAgentUpdate       = "agent.update"
	MethodAgentUpdateResult = "agent.updateResult"
	MethodAgentTraceResult  = "agent.traceResult"
)

type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      interface{} `json:"id,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type UpdateResultParams struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

type AgentManagedConfig struct {
	DisableAutoUpdate  bool    `json:"disable_auto_update"`
	Interval           float64 `json:"interval"`
	MonthRotate        int     `json:"month_rotate"`
	IncludeNics        string  `json:"include_nics"`
	ExcludeNics        string  `json:"exclude_nics"`
	IncludeMountpoints string  `json:"include_mountpoints"`
	MemoryIncludeCache bool    `json:"memory_include_cache"`
	GetIPAddrFromNic   bool    `json:"get_ip_addr_from_nic"`
}

type ConfigSetParams struct {
	Revision uint64             `json:"revision"`
	Config   AgentManagedConfig `json:"config"`
}

type ConfigReportParams struct {
	Revision uint64             `json:"revision"`
	Status   string             `json:"status"`
	Config   AgentManagedConfig `json:"config"`
	Error    string             `json:"error,omitempty"`
}

type Event struct {
	ID        string      `json:"id"`
	Method    string      `json:"method"`
	Params    interface{} `json:"params,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	ExpiresAt time.Time   `json:"expires_at"`
}

type EventResult struct {
	Status string  `json:"status,omitempty"`
	Events []Event `json:"events,omitempty"`
}

func NewNotification(method string, params interface{}) []byte {
	payload, _ := json.Marshal(Request{JSONRPC: Version, Method: method, Params: params})
	return payload
}

func NewRequest(id interface{}, method string, params interface{}) []byte {
	payload, _ := json.Marshal(Request{JSONRPC: Version, Method: method, Params: params, ID: id})
	return payload
}

func BuildReportPayload(report []byte, capabilities []string) []byte {
	return NewNotification(MethodAgentReport, reportParams{Report: json.RawMessage(report), Capabilities: capabilities})
}

func BuildReportRequest(id interface{}, report []byte, ackEventIDs []string, capabilities []string) []byte {
	return NewRequest(id, MethodAgentReport, reportParams{Report: json.RawMessage(report), AckEventIDs: ackEventIDs, Capabilities: capabilities})
}

func BuildBasicInfoPayload(info map[string]interface{}) []byte {
	return NewNotification(MethodAgentBasicInfo, map[string]interface{}{"info": info})
}

type reportParams struct {
	Report       json.RawMessage `json:"report"`
	AckEventIDs  []string        `json:"ack_event_ids,omitempty"`
	Capabilities []string        `json:"capabilities,omitempty"`
}

func BuildPingResultPayload(taskID uint, pingType string, value int, finishedAt time.Time) interface{} {
	return Request{
		JSONRPC: Version,
		Method:  MethodAgentPingResult,
		Params: map[string]interface{}{
			"task_id":     taskID,
			"ping_type":   pingType,
			"value":       value,
			"finished_at": finishedAt.Format(time.RFC3339Nano),
		},
	}
}

func BuildTraceResultPayload(result TraceResult) Request {
	return Request{JSONRPC: Version, Method: MethodAgentTraceResult, Params: result}
}

func BindParams(raw interface{}, target interface{}) error {
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}

func BindResult(raw interface{}, target interface{}) error {
	return BindParams(raw, target)
}
