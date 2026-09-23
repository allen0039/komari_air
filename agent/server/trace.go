package server

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	v2 "github.com/komari-monitor/komari-agent/protocol/v2"
	"github.com/komari-monitor/komari-agent/ws"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

var traceSemaphore = make(chan struct{}, 1)

const nextTraceVersion = "1.7.1"

var nextTraceBuilds = map[string]struct {
	url    string
	sha256 string
}{
	"amd64": {
		url:    "https://github.com/nxtrace/NTrace-core/releases/download/v1.7.1/nexttrace-tiny_linux_amd64",
		sha256: "093849f1012b065c29d307b8e47fedec667206829c14e105f83a852f60c628d1",
	},
	"arm64": {
		url:    "https://github.com/nxtrace/NTrace-core/releases/download/v1.7.1/nexttrace-tiny_linux_arm64",
		sha256: "8b134f6c6a7864b1ecc98b1f7cfae1d058ef6dcf8f0da862e3260752ce1858bd",
	},
}

func NewTraceTask(conn *ws.SafeConn, p v2.NextTraceParams) {
	if p.TaskID == "" || p.TargetHost == "" {
		return
	}
	go func() {
		log.Printf("trace task queued: task=%s target=%s host=%s", p.TaskID, p.TargetID, p.TargetHost)
		traceSemaphore <- struct{}{}
		defer func() { <-traceSemaphore }()
		log.Printf("trace task started: task=%s target=%s", p.TaskID, p.TargetID)
		result := runTrace(p)
		log.Printf("trace task finished: task=%s target=%s ok=%t hops=%d error=%q", p.TaskID, p.TargetID, result.OK, len(result.Hops), result.Error)
		// HTTP gives a receipt from the server, unlike a WS notification.
		payload := v2.BuildTraceResultPayload(result)
		payload.ID = result.TaskID
		for attempt := 0; attempt < 3; attempt++ {
			if err := postV2RPC(payload); err == nil {
				log.Printf("trace result acknowledged: task=%s", result.TaskID)
				return
			}
			log.Printf("trace result upload failed: task=%s attempt=%d", result.TaskID, attempt+1)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
			}
		}
		log.Printf("trace result upload exhausted: task=%s", result.TaskID)
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
	if hops, err := nextTraceIPv4(host); err == nil && len(hops) > 0 {
		r.Protocol = v2.TraceProtocolTCP
		r.Hops = hops
		r.OK = true
		r.FinishedAt = time.Now().UTC()
		return r
	} else {
		r.Protocol = v2.TraceProtocolTCP
		if err == nil {
			r.Error = "NextTrace returned no hops"
		} else {
			r.Error = fmt.Sprintf("NextTrace failed: %v", err)
		}
	}
	r.FinishedAt = time.Now().UTC()
	return r
}

type nextTraceOutput struct {
	Hops [][]nextTraceHop `json:"Hops"`
}

type nextTraceHop struct {
	Success bool `json:"Success"`
	Address *struct {
		IP string `json:"IP"`
	} `json:"Address"`
	Hostname string `json:"Hostname"`
	TTL      int    `json:"TTL"`
	RTT      int64  `json:"RTT"`
	Geo      *struct {
		ASN      string `json:"asnumber"`
		Country  string `json:"country"`
		Province string `json:"prov"`
		City     string `json:"city"`
		Owner    string `json:"owner"`
		ISP      string `json:"isp"`
		Whois    string `json:"whois"`
	} `json:"Geo"`
}

func nextTraceIPv4(host string) ([]v2.TraceHop, error) {
	tool, err := ensureNextTrace()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool,
		"-4", "-T", "-p", "80", "-q", "2", "--max-attempts", "2",
		"--timeout", "1500", "--no-rdns", "-j", host,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("nexttrace: %w", err)
	}
	return parseNextTrace(output)
}

func parseNextTrace(data []byte) ([]v2.TraceHop, error) {
	var result nextTraceOutput
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode nexttrace json: %w", err)
	}
	if len(result.Hops) == 0 {
		return nil, fmt.Errorf("nexttrace returned no hops")
	}
	hops := make([]v2.TraceHop, 0, len(result.Hops))
	for index, candidates := range result.Hops {
		added := false
		for _, candidate := range candidates {
			if !candidate.Success || candidate.Address == nil || candidate.Address.IP == "" {
				continue
			}
			hop := v2.TraceHop{Hop: index + 1}
			hop.Hop = candidate.TTL
			if hop.Hop <= 0 {
				hop.Hop = index + 1
			}
			hop.IP = candidate.Address.IP
			hop.Host = candidate.Hostname
			hop.Loss = 0
			hop.RTTMs = float64(candidate.RTT) / float64(time.Millisecond)
			if candidate.Geo != nil {
				hop.ASN = candidate.Geo.ASN
				if hop.Host == "" {
					hop.Host = strings.TrimSpace(strings.Join([]string{candidate.Geo.Owner, candidate.Geo.ISP, candidate.Geo.Whois}, " "))
				}
				hop.Location = strings.TrimSpace(strings.Join([]string{candidate.Geo.Country, candidate.Geo.Province, candidate.Geo.City}, " "))
			}
			hops = append(hops, hop)
			added = true
		}
		if !added {
			hops = append(hops, v2.TraceHop{Hop: index + 1, Loss: 100})
		}
	}
	return hops, nil
}

func ensureNextTrace() (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("nexttrace is only provisioned on linux")
	}
	build, ok := nextTraceBuilds[runtime.GOARCH]
	if !ok {
		return "", fmt.Errorf("unsupported nexttrace architecture %s", runtime.GOARCH)
	}
	// Reuse MiaoMiaoWu X's verified tool when both agents share a server.
	shared := filepath.Join(os.TempDir(), "mmwx-tools", "v"+nextTraceVersion, "nexttrace")
	if verifyExecutable(shared, build.sha256) == nil {
		return shared, nil
	}
	dir := filepath.Join(os.TempDir(), "komari-tools", "nexttrace", "v"+nextTraceVersion)
	path := filepath.Join(dir, "nexttrace")
	if verifyExecutable(path, build.sha256) == nil {
		return path, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 90 * time.Second}
	response, err := client.Get(build.url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download nexttrace: http %d", response.StatusCode)
	}
	tmp, err := os.CreateTemp(dir, ".nexttrace-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err = io.Copy(tmp, io.LimitReader(response.Body, 64<<20)); err != nil {
		tmp.Close()
		return "", err
	}
	if err = tmp.Close(); err != nil {
		return "", err
	}
	if err = verifyFile(tmpPath, build.sha256); err != nil {
		return "", err
	}
	if err = os.Chmod(tmpPath, 0o755); err != nil {
		return "", err
	}
	if err = os.Rename(tmpPath, path); err != nil {
		return "", err
	}
	return path, nil
}

func verifyExecutable(path, expected string) error {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		return fmt.Errorf("nexttrace executable unavailable")
	}
	return verifyFile(path, expected)
}

func verifyFile(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return fmt.Errorf("nexttrace checksum mismatch")
	}
	return nil
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
		deadline := time.Now().Add(minDuration(900*time.Millisecond, timeout))
		_ = conn.SetReadDeadline(deadline)
		matched := false
		for time.Now().Before(deadline) {
			buf := make([]byte, 1500)
			n, peer, readErr := conn.ReadFrom(buf)
			if readErr != nil {
				break
			}
			msg, parseErr := icmp.ParseMessage(1, buf[:n])
			if parseErr != nil || !matchesTraceReply(msg, target, port) {
				continue
			}
			ipStr := ""
			if addr, ok := peer.(*net.IPAddr); ok {
				ipStr = addr.IP.String()
			}
			hop := v2.TraceHop{Hop: ttl, IP: ipStr, RTTMs: float64(time.Since(start).Microseconds()) / 1000}
			if names, lookupErr := net.LookupAddr(ipStr); lookupErr == nil && len(names) > 0 {
				hop.Host = strings.TrimSuffix(names[0], ".")
			}
			hops = append(hops, hop)
			matched = true
			if ipStr == target.String() {
				return hops, nil
			}
			break
		}
		if !matched {
			if ctx.Err() != nil {
				break
			}
			hops = append(hops, v2.TraceHop{Hop: ttl, Loss: 100})
		}
	}
	return hops, nil
}

func matchesTraceReply(msg *icmp.Message, target net.IP, port int) bool {
	var data []byte
	switch body := msg.Body.(type) {
	case *icmp.TimeExceeded:
		data = body.Data
	case *icmp.DstUnreach:
		data = body.Data
	default:
		return false
	}
	header, err := ipv4.ParseHeader(data)
	if err != nil || header.Len < 20 || len(data) < header.Len+8 || !header.Dst.Equal(target) {
		return false
	}
	return int(binary.BigEndian.Uint16(data[header.Len+2:header.Len+4])) == port
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
