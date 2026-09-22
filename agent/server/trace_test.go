package server

import (
	"encoding/binary"
	"net"
	"testing"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func traceICMPMessage(t *testing.T, target string, port int) *icmp.Message {
	t.Helper()
	header := &ipv4.Header{Version: 4, Len: 20, TotalLen: 28, TTL: 1, Protocol: 17, Src: net.ParseIP("192.0.2.10").To4(), Dst: net.ParseIP(target).To4()}
	encoded, err := header.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	udp := make([]byte, 8)
	binary.BigEndian.PutUint16(udp[2:4], uint16(port))
	return &icmp.Message{Type: ipv4.ICMPTypeTimeExceeded, Body: &icmp.TimeExceeded{Data: append(encoded, udp...)}}
}

func TestMatchesTraceReply(t *testing.T) {
	target := net.ParseIP("202.96.199.133").To4()
	if !matchesTraceReply(traceICMPMessage(t, target.String(), 33435), target, 33435) {
		t.Fatal("expected matching trace reply")
	}
	if matchesTraceReply(traceICMPMessage(t, "1.1.1.1", 33435), target, 33435) {
		t.Fatal("accepted reply for another target")
	}
	if matchesTraceReply(traceICMPMessage(t, target.String(), 33436), target, 33435) {
		t.Fatal("accepted reply for another UDP port")
	}
}
