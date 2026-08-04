package handler

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestChannelMonitorUserResponsesHideInternalMonitorName(t *testing.T) {
	listPayload, err := json.Marshal(userMonitorViewToItem(&service.UserMonitorView{
		ID:        1,
		Name:      "private supplier relay",
		Provider:  "openai",
		GroupName: "A0001-public",
	}))
	if err != nil {
		t.Fatalf("marshal monitor list item: %v", err)
	}
	assertNoPublicMonitorName(t, listPayload)

	detailPayload, err := json.Marshal(userMonitorDetailToResponse(&service.UserMonitorDetail{
		ID:        1,
		Name:      "private supplier relay",
		Provider:  "openai",
		GroupName: "A0001-public",
	}))
	if err != nil {
		t.Fatalf("marshal monitor detail: %v", err)
	}
	assertNoPublicMonitorName(t, detailPayload)
}

func assertNoPublicMonitorName(t *testing.T, payload []byte) {
	t.Helper()
	var fields map[string]any
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal public monitor response: %v", err)
	}
	for _, hidden := range []string{"name", "supplier_id", "supplier_name", "endpoint", "api_key"} {
		if _, exists := fields[hidden]; exists {
			t.Fatalf("public monitor response exposed %q: %s", hidden, payload)
		}
	}
}
