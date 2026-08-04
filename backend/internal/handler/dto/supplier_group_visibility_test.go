package dto

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestSupplierGroupPublicMapperReturnsOnlyFinalRate(t *testing.T) {
	supplierID := int64(9)
	adjustment := 0.01
	group := &service.Group{
		ID:                      17,
		Name:                    "A0009-resource",
		Description:             "private supplier relay",
		Platform:                service.PlatformOpenAI,
		RateMultiplier:          0.05,
		EffectiveRateMultiplier: 0.05,
		SupplierID:              &supplierID,
		SupplierAdminAdjustment: &adjustment,
	}

	public := GroupFromServiceShallow(group)
	if public.Description != "" {
		t.Fatalf("public supplier group description = %q, want empty", public.Description)
	}
	if public.RateMultiplier != 0.05 {
		t.Fatalf("public supplier group rate = %v, want final rate 0.05", public.RateMultiplier)
	}
	raw, err := json.Marshal(public)
	if err != nil {
		t.Fatalf("marshal public group: %v", err)
	}
	for _, forbidden := range []string{"supplier_id", "supplier_admin_adjustment", "effective_rate_multiplier", "private supplier relay"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("public supplier group contains %q: %s", forbidden, raw)
		}
	}

	admin := GroupFromServiceAdmin(group)
	if admin.Description != "private supplier relay" || admin.RateMultiplier != 0.05 {
		t.Fatalf("admin supplier group lost internal fields: %+v", admin)
	}
}
