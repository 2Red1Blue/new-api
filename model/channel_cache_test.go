package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetChannelWithExclusionsReturnsNilWhenPriorityFullyExcluded(t *testing.T) {
	truncateTables(t)

	originalMainDBType := common.MainDatabaseType()
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		common.SetMainDatabaseType(originalMainDBType)
	})

	priorityHigh := int64(10)
	priorityLow := int64(5)

	channels := []*Channel{
		{Id: 1, Name: "high", Group: "default", Models: "gpt-test", Status: common.ChannelStatusEnabled, Priority: &priorityHigh},
		{Id: 2, Name: "low", Group: "default", Models: "gpt-test", Status: common.ChannelStatusEnabled, Priority: &priorityLow},
	}
	for _, channel := range channels {
		require.NoError(t, DB.Create(channel).Error)
	}

	abilities := []*Ability{
		{Group: "default", Model: "gpt-test", ChannelId: 1, Enabled: true, Priority: &priorityHigh},
		{Group: "default", Model: "gpt-test", ChannelId: 2, Enabled: true, Priority: &priorityLow},
	}
	for _, ability := range abilities {
		require.NoError(t, DB.Create(ability).Error)
	}

	channel, err := GetChannelWithExclusions("default", "gpt-test", 0, "", map[int]struct{}{1: {}})
	require.NoError(t, err)
	require.Nil(t, channel)
}
