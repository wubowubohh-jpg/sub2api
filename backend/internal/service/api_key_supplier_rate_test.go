package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type apiKeySupplierRateSettingRepo struct {
	value string
}

func (r *apiKeySupplierRateSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (r *apiKeySupplierRateSettingRepo) GetValue(context.Context, string) (string, error) {
	return r.value, nil
}
func (r *apiKeySupplierRateSettingRepo) Set(context.Context, string, string) error { return nil }
func (r *apiKeySupplierRateSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}
func (r *apiKeySupplierRateSettingRepo) SetMultiple(context.Context, map[string]string) error {
	return nil
}
func (r *apiKeySupplierRateSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return nil, nil
}
func (r *apiKeySupplierRateSettingRepo) Delete(context.Context, string) error { return nil }

func TestAPIKeyServiceDecoratesSupplierGroupWithFinalRate(t *testing.T) {
	supplierID := int64(3)
	service := &APIKeyService{
		settingService: NewSettingService(&apiKeySupplierRateSettingRepo{value: "0.01"}, &config.Config{}),
	}

	group := &Group{RateMultiplier: 0.04, SupplierID: &supplierID}
	service.decorateSupplierGroupRate(context.Background(), group)
	if group.RateMultiplier != 0.05 || group.EffectiveRateMultiplier != 0.05 {
		t.Fatalf("global adjusted rates = (%v, %v), want final rate 0.05", group.RateMultiplier, group.EffectiveRateMultiplier)
	}

	localAdjustment := 0.02
	group.RateMultiplier = 0.04
	group.SupplierAdminAdjustment = &localAdjustment
	service.decorateSupplierGroupRate(context.Background(), group)
	if group.RateMultiplier != 0.06 || group.EffectiveRateMultiplier != 0.06 {
		t.Fatalf("local adjusted rates = (%v, %v), want final rate 0.06", group.RateMultiplier, group.EffectiveRateMultiplier)
	}
}
