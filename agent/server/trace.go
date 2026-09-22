package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	v2 "github.com/komari-monitor/komari-agent/protocol/v2"
	"github.com/komari-monitor/komari-agent/ws"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

var traceSemaphore = make(chan struct{}, 1)

func NewTraceTask(conn *ws.SafeConn, p v2.NextTraceParams) {
	if p.TaskID == "" || p.TargetHost == "" {
		return
	}
	go func() {
		traceSemaphore <- struct{}{}
		defer func() { <-traceSemaphore }()
		result := runTrace(p)
		payload := v2.BuildTraceResultPayload(result)
		if conn != nil {
			if err := conn.WriteJSON(payload); err != nil {
				log.Printf("failed to send trace result: %v", err)
			}
			return
		}
		if err := postV2RPC(payload); err != nil {
			log.Printf("failed to upload trace result: %v", err)
		}
	}()
}

func runTrace(p v2.NextTraceParams) v2.TraceResult {
	started := time.Now().UTC()
	r := v2.TraceResult{TaskID: p.TaskID, SourceID: p.SourceID, TargetID: p.TargetID, TargetHost: p.TargetHost, IPFamily: v2.IPFamilyIPv4, Protocol: v2.TraceProtocolICMP, StartedAt: started, Hops: make([]v2.TraceHop, 0, 30)}
	if p.IPFamily != "" && p.IPFamily != v2.IPFamilyIPv4 {
		r.Error = "only ipv4 trace is supported"
		r.FinishedAt = time.Now().UTC()
		return r
	}
	host := strings.Trim(p.TargetHost, "[]")
	ip := net.ParseIP(host)
	if ip == nil {
		addrs, err := net.LookupIP(host)
		if err != nil {
			r.Error = err.Error()
			r.FinishedAt = time.Now().UTC()
			return r
		}
		for _, candidate := range addrs {
			if candidate.To4() != nil {
				ip = candidate.To4()
				break
			}
		}
	}
	if ip == nil || ip.To4() == nil {
		r.Error = "target has no ipv4 address"
		r.FinishedAt = time.Now().UTC()
		return r
	}
	maxHops := p.MaxHops
	if maxHops <= 0 || maxHops > 30 {
		maxHops = 30
	}
	timeout := time.Duration(p.TimeoutMs) * time.Millisecond
	if timeout <= 0 || timeout > 20*time.Second {
		timeout = 20 * time.Second
	}
	hops, err := nativeIPv4Trace(ip.To4(), maxHops, timeout)
	r.Hops = hops
	r.OK = err == nil && len(hops) > 0
	if err != nil {
		r.Error = err.Error()
	}
	r.FinishedAt = time.Now().UTC()
	return r
}

func nativeIPv4Trace(target net.IP, maxHops int, timeout time.Duration) ([]v2.TraceHop, error) {
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("icmp socket: %w", err)
	}
	defer conn.Close()
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, err
	}
	defer udp.Close()
	packet := ipv4.NewPacketConn(udp)
	hops := make([]v2.TraceHop, 0, maxHops)
	basePort := 33434
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for ttl := 1; ttl <= maxHops; ttl++ {
		if err := packet.SetTTL(ttl); err != nil {
			return hops, err
		}
		port := basePort + ttl
		start := time.Now()
		if _, err := udp.WriteTo([]byte("komari-trace"), &net.UDPAddr{IP: target, Port: port}); err != nil {
			return hops, err
		}
		_ = conn.SetReadDeadline(time.Now().Add(minDuration(900*time.Millisecond, timeout)))
		buf := make([]byte, 1500)
		n, peer, err := conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			hops = append(hops, v2.TraceHop{Hop: ttl, Loss: 100})
			continue
		}
		msg, err := icmp.ParseMessage(1, buf[:n])
		if err != nil {
			continue
		}
		ipStr := ""
		if addr, ok := peer.(*net.IPAddr); ok {
			ipStr = addr.IP.String()
		}
		hop := v2.TraceHop{Hop: ttl, IP: ipStr, RTTMs: float64(time.Since(start).Microseconds()) / 1000}
		// Reverse DNS is the same signal used by common route probes (including
		// nexttrace) and preserves carrier names such as chinanet/cmcc.
		if names, lookupErr := net.LookupAddr(ipStr); lookupErr == nil && len(names) > 0 {
			hop.Host = strings.TrimSuffix(names[0], ".")
		}
		hops = append(hops, hop)
		if msg.Type == ipv4.ICMPTypeEchoReply || ipStr == target.String() {
			break
		}
	}
	return hops, nil
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
