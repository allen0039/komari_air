package v2

import "time"

const MethodNetworkTestNextTrace = "networkTest.nextTrace"

type IPFamily string

const IPFamilyIPv4 IPFamily = "ipv4"

type TraceProtocol string

const (
	TraceProtocolICMP TraceProtocol = "icmp"
	TraceProtocolTCP  TraceProtocol = "tcp"
)

type NextTraceParams struct {
	TaskID     string        `json:"task_id"`
	SourceID   string        `json:"source_id"`
	TargetID   string        `json:"target_id"`
	TargetHost string        `json:"target_host"`
	IPFamily   IPFamily      `json:"ip_family"`
	Protocol   TraceProtocol `json:"protocol"`
	MaxHops    int           `json:"max_hops"`
	TimeoutMs  int           `json:"timeout_ms"`
	RawOutput  bool          `json:"raw_output"`
}

type TraceResult struct {
	TaskID     string        `json:"task_id"`
	SourceID   string        `json:"source_id"`
	TargetID   string        `json:"target_id"`
	TargetHost string        `json:"target_host"`
	IPFamily   IPFamily      `json:"ip_family"`
	Protocol   TraceProtocol `json:"protocol"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt time.Time     `json:"finished_at"`
	OK         bool          `json:"ok"`
	Error      string        `json:"error,omitempty"`
	Hops       []TraceHop    `json:"hops"`
	Truncated  bool          `json:"truncated,omitempty"`
}

type TraceHop struct {
	Hop      int     `json:"hop"`
	IP       string  `json:"ip"`
	Host     string  `json:"host,omitempty"`
	ASN      string  `json:"asn,omitempty"`
	Location string  `json:"location,omitempty"`
	RTTMs    float64 `json:"rtt_ms"`
	Loss     float64 `json:"loss"`
}
