package telegram

import (
	"testing"

	"lead-scout/internal/core"
)

func TestParseAction(t *testing.T) {
	action, err := ParseAction("lead:123:saved")
	if err != nil {
		t.Fatal(err)
	}
	if action.LeadID != 123 || action.State != core.StateSaved {
		t.Fatalf("action = %+v", action)
	}
}

func TestParseActionRejectsUnknownState(t *testing.T) {
	if _, err := ParseAction("lead:123:new"); err == nil {
		t.Fatal("expected error for unsupported callback state")
	}
}
