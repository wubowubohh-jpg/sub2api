package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSupplierResourceRateDetails(t *testing.T) {
	probeRate := 0.06
	invalidProbeRate := math.NaN()
	tests := []struct {
		name            string
		configuredRate  float64
		probeEnabled    bool
		upstreamRate    *float64
		adminAdjustment float64
		wantSource      string
		wantApplied     float64
		wantEffective   float64
	}{
		{
			name:           "probe rate remains informational",
			configuredRate: 0.04, probeEnabled: true, upstreamRate: &probeRate,
			adminAdjustment: 0.01, wantSource: "configured", wantApplied: 0.04, wantEffective: 0.05,
		},
		{
			name:           "configured rate when probe is disabled",
			configuredRate: 0.04, probeEnabled: false, upstreamRate: &probeRate,
			adminAdjustment: 0.01, wantSource: "configured", wantApplied: 0.04, wantEffective: 0.05,
		},
		{
			name:           "configured rate when probe result is invalid",
			configuredRate: 0.04, probeEnabled: true, upstreamRate: &invalidProbeRate,
			adminAdjustment: 0.01, wantSource: "configured", wantApplied: 0.04, wantEffective: 0.05,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, applied, effective := supplierResourceRateDetails(
				tt.configuredRate, tt.probeEnabled, tt.upstreamRate, tt.adminAdjustment,
			)
			require.Equal(t, tt.wantSource, source)
			require.InDelta(t, tt.wantApplied, applied, 0.000001)
			require.InDelta(t, tt.wantEffective, effective, 0.000001)
		})
	}
}
