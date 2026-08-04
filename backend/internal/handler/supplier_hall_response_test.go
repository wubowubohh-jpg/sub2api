package handler

import (
	"encoding/json"
	"testing"
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
	for _, hidden := range []string{"supplier_id", "supplier_name", "base_rate", "admin_adjustment"} {
		if _, exists := fields[hidden]; exists {
			t.Fatalf("public supplier hall response exposed %q", hidden)
		}
	}
	if got, ok := fields["effective_rate"].(float64); !ok || got != 0.05 {
		t.Fatalf("effective_rate = %#v, want 0.05", fields["effective_rate"])
	}
}
