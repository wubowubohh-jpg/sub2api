package handler

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestSupplierHallItemDoesNotExposeSupplierOrRateBreakdown(t *testing.T) {
	payload, err := json.Marshal(supplierHallItem{
		ID:            1,
		Name:          "A0001-main",
		Platform:      "openai",
		EffectiveRate: 0.05,
		Status:        "active",
	})
	if err != nil {
		t.Fatalf("marshal supplier hall item: %v", err)
	}

	var fields map[string]any
	if err = json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal supplier hall item: %v", err)
	}
	for _, hidden := range []string{"supplier_id", "supplier_name", "base_rate", "admin_adjustment", "description", "monitor_status"} {
		if _, exists := fields[hidden]; exists {
			t.Fatalf("public supplier hall response exposed %q", hidden)
		}
	}
	if got, ok := fields["effective_rate"].(float64); !ok || got != 0.05 {
		t.Fatalf("effective_rate = %#v, want 0.05", fields["effective_rate"])
	}
}

func TestSortSupplierHallItemsPlacesHealthyGroupsFirst(t *testing.T) {
	items := []supplierHallItem{
		{ID: 1, Metrics: service.SupplierGroupMetrics{MonitorStatus: "failed"}},
		{ID: 2, Metrics: service.SupplierGroupMetrics{}},
		{ID: 3, Metrics: service.SupplierGroupMetrics{MonitorStatus: "operational"}},
		{ID: 4, Metrics: service.SupplierGroupMetrics{MonitorStatus: "degraded"}},
		{ID: 5, Metrics: service.SupplierGroupMetrics{MonitorStatus: "error"}},
		{ID: 6, Metrics: service.SupplierGroupMetrics{MonitorStatus: "operational"}},
	}

	sortSupplierHallItems(items)

	want := []int64{3, 4, 6, 1, 2, 5}
	for i, id := range want {
		if items[i].ID != id {
			t.Fatalf("items[%d].ID = %d, want %d", i, items[i].ID, id)
		}
	}
}
