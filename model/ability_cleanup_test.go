package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestCleanupModelsRemoveUpdatesChannelsAndDeletesAbilities(t *testing.T) {
	truncateTables(t)

	priority := int64(1)
	require.NoError(t, DB.Create(&[]Channel{
		{Id: 1, Name: "openai", Group: "default", Models: "gpt-5,gpt-5.2,gpt-5-chat-latest", Status: common.ChannelStatusEnabled, Priority: &priority},
		{Id: 2, Name: "claude", Group: "default", Models: "claude-haiku-4-5,claude-haiku-4-5-20251001", Status: common.ChannelStatusEnabled, Priority: &priority},
	}).Error)
	require.NoError(t, DB.Create(&[]Ability{
		{Group: "default", Model: "gpt-5", ChannelId: 1, Enabled: true, Priority: &priority},
		{Group: "default", Model: "gpt-5.2", ChannelId: 1, Enabled: true, Priority: &priority},
		{Group: "default", Model: "claude-haiku-4-5", ChannelId: 2, Enabled: true, Priority: &priority},
		{Group: "default", Model: "claude-haiku-4-5-20251001", ChannelId: 2, Enabled: true, Priority: &priority},
	}).Error)

	result, err := CleanupModelsFromChannelsAndAbilities([]string{"gpt-5", "claude-haiku-4-5"}, ModelCleanupModeRemove)
	require.NoError(t, err)
	require.Equal(t, int64(2), result.UpdatedChannels)
	require.Equal(t, int64(2), result.DeletedAbilities)
	require.ElementsMatch(t, []int{1, 2}, result.MatchedChannelIDs)

	var openAI Channel
	require.NoError(t, DB.First(&openAI, "id = ?", 1).Error)
	require.Equal(t, "gpt-5.2,gpt-5-chat-latest", openAI.Models)

	var claude Channel
	require.NoError(t, DB.First(&claude, "id = ?", 2).Error)
	require.Equal(t, "claude-haiku-4-5-20251001", claude.Models)

	var remaining []string
	require.NoError(t, DB.Model(&Ability{}).Order("model").Pluck("model", &remaining).Error)
	require.Equal(t, []string{"claude-haiku-4-5-20251001", "gpt-5.2"}, remaining)
}

func TestCleanupModelsDisableRemovesChannelModelsAndDisablesAbilities(t *testing.T) {
	truncateTables(t)

	priority := int64(1)
	require.NoError(t, DB.Create(&Channel{
		Id:       1,
		Name:     "openai",
		Group:    "default",
		Models:   "gpt-5,gpt-5.2",
		Status:   common.ChannelStatusEnabled,
		Priority: &priority,
	}).Error)
	require.NoError(t, DB.Create(&[]Ability{
		{Group: "default", Model: "gpt-5", ChannelId: 1, Enabled: true, Priority: &priority},
		{Group: "default", Model: "gpt-5.2", ChannelId: 1, Enabled: true, Priority: &priority},
	}).Error)

	result, err := CleanupModelsFromChannelsAndAbilities([]string{"gpt-5"}, ModelCleanupModeDisable)
	require.NoError(t, err)
	require.Equal(t, int64(1), result.UpdatedChannels)
	require.Equal(t, int64(1), result.DisabledAbilities)

	var channel Channel
	require.NoError(t, DB.First(&channel, "id = ?", 1).Error)
	require.Equal(t, "gpt-5.2", channel.Models)

	var ability Ability
	require.NoError(t, DB.First(&ability, "model = ?", "gpt-5").Error)
	require.False(t, ability.Enabled)
}
