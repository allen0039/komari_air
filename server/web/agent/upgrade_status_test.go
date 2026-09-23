package agent

import "testing"

func TestUpgradeStatusTracksSuccessfulResult(t *testing.T) {
	ResetUpgradeStatuses("0.1.9", []UpgradeStatus{{
		UUID:           "node-1",
		Name:           "node one",
		CurrentVersion: "0.1.8",
	}})
	t.Cleanup(FinishUpgradeRun)

	ApplyAgentUpdateResult("node-1", "checking", "0.1.8", "")
	ApplyAgentUpdateResult("node-1", "installed", "0.1.8", "")
	if UpgradeStatusIsTerminal("node-1") {
		t.Fatal("installed result must wait for reconnect confirmation")
	}
	if !SetUpgradeStatus("node-1", UpgradeStateSucceeded, "confirmed", "0.1.9") {
		t.Fatal("expected reconnect confirmation to update status")
	}

	item := GetUpgradeStatusSnapshot().Items["node-1"]
	if item.State != UpgradeStateSucceeded || item.CurrentVersion != "0.1.9" {
		t.Fatalf("unexpected final status: %+v", item)
	}
}

func TestUpgradeStatusReportsFailureAndDoesNotRegress(t *testing.T) {
	ResetUpgradeStatuses("0.1.9", []UpgradeStatus{{UUID: "node-2", CurrentVersion: "0.1.8"}})
	t.Cleanup(FinishUpgradeRun)

	ApplyAgentUpdateResult("node-2", "failed", "0.1.8", "download failed")
	if SetUpgradeStatus("node-2", UpgradeStateWaiting, "late update", "") {
		t.Fatal("terminal failure must not be overwritten")
	}

	item := GetUpgradeStatusSnapshot().Items["node-2"]
	if item.State != UpgradeStateFailed || item.Message != "download failed" {
		t.Fatalf("unexpected failure status: %+v", item)
	}
}

func TestUpgradeStatusMarksMatchingAgentUpToDate(t *testing.T) {
	ResetUpgradeStatuses("0.1.9", []UpgradeStatus{{UUID: "node-3", CurrentVersion: "0.1.9"}})
	t.Cleanup(FinishUpgradeRun)

	ApplyAgentUpdateResult("node-3", "up_to_date", "0.1.9", "")
	item := GetUpgradeStatusSnapshot().Items["node-3"]
	if item.State != UpgradeStateSucceeded {
		t.Fatalf("expected success, got %+v", item)
	}
}
