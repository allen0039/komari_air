package server

import "testing"

func TestV2CapabilitiesExcludeRemovedRemoteControl(t *testing.T) {
	for _, capability := range v2Capabilities {
		if capability == "terminal" || capability == "file" || capability == "exec" {
			t.Fatalf("removed capability is still advertised: %s", capability)
		}
	}

	want := map[string]bool{"ping": true, "message": true, "event": true, "config:v1": true}
	for _, capability := range v2Capabilities {
		delete(want, capability)
	}
	if len(want) != 0 {
		t.Fatalf("required capabilities are missing: %v", want)
	}
}
