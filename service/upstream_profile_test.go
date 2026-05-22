package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestMatchInsufficientBalanceKeywordDefaultChineseQuota(t *testing.T) {
	keyword, ok := matchInsufficientBalanceKeyword("status_code=403, 用户额度不足, 剩余额度: ＄0.000000", nil)
	require.True(t, ok)
	require.Equal(t, "额度不足", keyword)
}

func TestMatchInsufficientBalanceKeywordProfileAddsToDefaults(t *testing.T) {
	profile := &model.ChannelUpstreamProfile{
		InsufficientBalanceKeywords: "custom-empty-wallet",
	}

	defaultKeyword, defaultOK := matchInsufficientBalanceKeyword("status_code=403, 用户额度不足", profile)
	customKeyword, customOK := matchInsufficientBalanceKeyword("status_code=402, custom-empty-wallet", profile)

	require.True(t, defaultOK)
	require.Equal(t, "额度不足", defaultKeyword)
	require.True(t, customOK)
	require.Equal(t, "custom-empty-wallet", customKeyword)
}

func TestMatchInsufficientBalanceKeywordIncludesGlobalDisableKeywords(t *testing.T) {
	orig := operation_setting.AutomaticDisableKeywords
	t.Cleanup(func() {
		operation_setting.AutomaticDisableKeywords = orig
	})
	operation_setting.AutomaticDisableKeywords = []string{"global-empty-wallet"}

	keyword, ok := matchInsufficientBalanceKeyword("upstream says global-empty-wallet", nil)

	require.True(t, ok)
	require.Equal(t, "global-empty-wallet", keyword)
}
