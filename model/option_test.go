package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestParseQuotaOptionValueSupportsDecimalDisplayAmounts(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	tests := []struct {
		name  string
		value string
		want  int
	}{
		{name: "decimal half unit", value: "0.5", want: 250000},
		{name: "decimal tenth unit", value: "0.1", want: 50000},
		{name: "integer remains raw quota", value: "500000", want: 500000},
		{name: "empty value", value: "", want: 0},
		{name: "invalid decimal", value: "0.5x", want: 0},
		{name: "negative integer", value: "-1", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, parseQuotaOptionValue(tt.value))
		})
	}
}

func TestSetQuotaOptionValueNormalizesOptionMap(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	originalOptionMap := common.OptionMap
	common.QuotaPerUnit = 500000
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
		common.OptionMap = originalOptionMap
	})

	var quota int
	setQuotaOptionValue("QuotaForNewUser", "0.5", &quota)

	require.Equal(t, 250000, quota)
	require.Equal(t, "250000", common.OptionMap["QuotaForNewUser"])
}
