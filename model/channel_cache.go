package model

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

var group2model2channels map[string]map[string][]int // enabled channel
var channelsIDM map[int]*Channel                     // all channels include disabled
// channel2advancedCustomConfig caches parsed Advanced Custom (type 58) configs so
// path-aware selection avoids re-parsing JSON per request. Refreshed on full sync.
var channel2advancedCustomConfig map[int]*dto.AdvancedCustomConfig
var channelSyncLock sync.RWMutex

type ChannelSelectionCandidate struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Priority int64  `json:"priority"`
	Weight   int    `json:"weight"`
	Excluded bool   `json:"excluded,omitempty"`
}

type ChannelSelectionSnapshot struct {
	Group              string                      `json:"group"`
	Model              string                      `json:"model"`
	Retry              int                         `json:"retry"`
	RequestPath        string                      `json:"request_path,omitempty"`
	MatchedModel       string                      `json:"matched_model,omitempty"`
	TargetPriority     int64                       `json:"target_priority,omitempty"`
	Priorities         []int                       `json:"priorities,omitempty"`
	TotalMatched       int                         `json:"total_matched"`
	Candidates         []ChannelSelectionCandidate `json:"candidates,omitempty"`
	ExcludedCandidates []ChannelSelectionCandidate `json:"excluded_candidates,omitempty"`
}

func InitChannelCache() {
	if !common.MemoryCacheEnabled {
		return
	}
	newChannelId2channel := make(map[int]*Channel)
	newChannel2advancedCustomConfig := make(map[int]*dto.AdvancedCustomConfig)
	var channels []*Channel
	DB.Find(&channels)
	for _, channel := range channels {
		newChannelId2channel[channel.Id] = channel
		if channel.Type == constant.ChannelTypeAdvancedCustom {
			if config := channel.GetOtherSettings().AdvancedCustom; config != nil {
				newChannel2advancedCustomConfig[channel.Id] = config
			}
		}
	}
	var abilities []*Ability
	DB.Find(&abilities)
	groups := make(map[string]bool)
	for _, ability := range abilities {
		groups[ability.Group] = true
	}
	newGroup2model2channels := make(map[string]map[string][]int)
	for group := range groups {
		newGroup2model2channels[group] = make(map[string][]int)
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue // skip disabled channels
		}
		groups := strings.Split(channel.Group, ",")
		for _, group := range groups {
			models := strings.Split(channel.Models, ",")
			for _, model := range models {
				if _, ok := newGroup2model2channels[group][model]; !ok {
					newGroup2model2channels[group][model] = make([]int, 0)
				}
				newGroup2model2channels[group][model] = append(newGroup2model2channels[group][model], channel.Id)
			}
		}
	}

	// sort by priority
	for group, model2channels := range newGroup2model2channels {
		for model, channels := range model2channels {
			sort.Slice(channels, func(i, j int) bool {
				return newChannelId2channel[channels[i]].GetPriority() > newChannelId2channel[channels[j]].GetPriority()
			})
			newGroup2model2channels[group][model] = channels
		}
	}

	channelSyncLock.Lock()
	group2model2channels = newGroup2model2channels
	//channelsIDM = newChannelId2channel
	for i, channel := range newChannelId2channel {
		if channel.ChannelInfo.IsMultiKey {
			channel.Keys = channel.GetKeys()
			if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
				if oldChannel, ok := channelsIDM[i]; ok {
					// 存在旧的渠道，如果是多key且轮询，保留轮询索引信息
					if oldChannel.ChannelInfo.IsMultiKey && oldChannel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
						channel.ChannelInfo.MultiKeyPollingIndex = oldChannel.ChannelInfo.MultiKeyPollingIndex
					}
				}
			}
		}
	}
	channelsIDM = newChannelId2channel
	channel2advancedCustomConfig = newChannel2advancedCustomConfig
	channelSyncLock.Unlock()
	common.SysLog("channels synced from database")
}

func SyncChannelCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing channels from database")
		InitChannelCache()
	}
}

func GetRandomSatisfiedChannel(group string, model string, retry int, requestPath string) (*Channel, error) {
	return GetRandomSatisfiedChannelWithExclusions(group, model, retry, requestPath, nil)
}

func GetRandomSatisfiedChannelWithExclusions(group string, model string, retry int, requestPath string, excludedChannelIDs map[int]struct{}) (*Channel, error) {
	// if memory cache is disabled, get channel directly from database
	if !common.MemoryCacheEnabled {
		return GetChannelWithExclusions(group, model, retry, requestPath, excludedChannelIDs)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	// First, try to find channels with the exact model name.
	channels := filterChannelsByRequestPath(group2model2channels[group][model], requestPath)

	// If no channels found, try to find channels with the normalized model name.
	if len(channels) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(model)
		channels = filterChannelsByRequestPath(group2model2channels[group][normalizedModel], requestPath)
	}

	// Filter out excluded channels; if none remain after exclusion, try fallback expansion.
	channels = filterExcluded(channels, excludedChannelIDs)
	if len(channels) == 0 {
		fallbackChannels := GetChannelsForGroupModelWithFallback(group, model)
		channels = filterChannelsByRequestPath(fallbackChannels, requestPath)
		channels = filterExcluded(channels, excludedChannelIDs)
	}

	if len(channels) == 0 {
		return nil, nil
	}

	if len(channels) == 1 {
		if channel, ok := channelsIDM[channels[0]]; ok {
			return channel, nil
		}
		return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channels[0])
	}

	uniquePriorities := make(map[int]bool)
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			uniquePriorities[int(channel.GetPriority())] = true
		} else {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}
	var sortedUniquePriorities []int
	for priority := range uniquePriorities {
		sortedUniquePriorities = append(sortedUniquePriorities, priority)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sortedUniquePriorities)))

	if retry >= len(uniquePriorities) {
		retry = len(uniquePriorities) - 1
	}
	targetPriority := int64(sortedUniquePriorities[retry])

	// get the priority for the given retry number
	var sumWeight = 0
	var targetChannels []*Channel
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			if channel.GetPriority() == targetPriority {
				sumWeight += channel.GetWeight()
				targetChannels = append(targetChannels, channel)
			}
		} else {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}

	if len(targetChannels) == 0 {
		return nil, nil
	}

	// smoothing factor and adjustment
	smoothingFactor := 1
	smoothingAdjustment := 0

	if sumWeight == 0 {
		// when all channels have weight 0, set sumWeight to the number of channels and set smoothing adjustment to 100
		// each channel's effective weight = 100
		sumWeight = len(targetChannels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(targetChannels) < 10 {
		// when the average weight is less than 10, set smoothing factor to 100
		smoothingFactor = 100
	}

	// Calculate the total weight of all channels up to endIdx
	totalWeight := sumWeight * smoothingFactor

	// Generate a random value in the range [0, totalWeight)
	randomWeight := rand.Intn(totalWeight)

	// Find a channel based on its weight
	for _, channel := range targetChannels {
		randomWeight -= channel.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return channel, nil
		}
	}
	// return null if no channel is not found
	return nil, errors.New("channel not found")
}

func GetChannelSelectionSnapshot(group string, model string, retry int, requestPath string, excludedChannelIDs map[int]struct{}) ChannelSelectionSnapshot {
	snapshot := ChannelSelectionSnapshot{
		Group:       group,
		Model:       model,
		Retry:       retry,
		RequestPath: requestPath,
	}
	if !common.MemoryCacheEnabled {
		return snapshot
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	matchedModel := model
	channels := filterChannelsByRequestPath(group2model2channels[group][model], requestPath)
	if len(channels) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(model)
		channels = filterChannelsByRequestPath(group2model2channels[group][normalizedModel], requestPath)
		if len(channels) > 0 {
			matchedModel = normalizedModel
		}
	}
	snapshot.MatchedModel = matchedModel
	snapshot.TotalMatched = len(channels)
	if len(channels) == 0 {
		return snapshot
	}

	uniquePriorities := make(map[int]bool)
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			uniquePriorities[int(channel.GetPriority())] = true
		}
	}
	for priority := range uniquePriorities {
		snapshot.Priorities = append(snapshot.Priorities, priority)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(snapshot.Priorities)))
	if retry >= len(snapshot.Priorities) {
		retry = len(snapshot.Priorities) - 1
	}
	if retry < 0 || retry >= len(snapshot.Priorities) {
		return snapshot
	}
	snapshot.TargetPriority = int64(snapshot.Priorities[retry])

	for _, channelId := range channels {
		channel, ok := channelsIDM[channelId]
		if !ok || channel.GetPriority() != snapshot.TargetPriority {
			continue
		}
		candidate := ChannelSelectionCandidate{
			ID:       channel.Id,
			Name:     channel.Name,
			Priority: channel.GetPriority(),
			Weight:   channel.GetWeight(),
		}
		if _, excluded := excludedChannelIDs[channelId]; excluded {
			candidate.Excluded = true
			snapshot.ExcludedCandidates = append(snapshot.ExcludedCandidates, candidate)
			continue
		}
		snapshot.Candidates = append(snapshot.Candidates, candidate)
	}
	return snapshot
}

// GetChannelPriorityLevelCount 返回指定分组和模型的优先级层数，供亲和性迁移遍历使用
func GetChannelPriorityLevelCount(group, modelName string) int {
	if !common.MemoryCacheEnabled {
		return 1 // DB 模式无法廉价计算，返回 1 表示只看最高优先级
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	channels := group2model2channels[group][modelName]
	if len(channels) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(modelName)
		channels = group2model2channels[group][normalizedModel]
		if len(channels) == 0 {
			return 0
		}
	}
	priorities := make(map[int]struct{})
	for _, id := range channels {
		if ch, ok := channelsIDM[id]; ok {
			priorities[int(ch.GetPriority())] = struct{}{}
		}
	}
	return len(priorities)
}

// filterChannelsByRequestPath restricts candidates by request path. Only Advanced
// Custom (type 58) channels are path-checked: they are kept only when one of their
// configured routes matches requestPath. All other channel types always pass.
// When requestPath is empty (non-relay callers) filtering is skipped.
// Caller must hold channelSyncLock (read lock). The cached slice is never mutated.
func filterChannelsByRequestPath(channels []int, requestPath string) []int {
	if requestPath == "" || len(channels) == 0 {
		return channels
	}
	filtered := make([]int, 0, len(channels))
	for _, channelId := range channels {
		channel, ok := channelsIDM[channelId]
		if !ok {
			// keep it so the downstream consistency error is raised as before
			filtered = append(filtered, channelId)
			continue
		}
		if channel.Type != constant.ChannelTypeAdvancedCustom {
			filtered = append(filtered, channelId)
			continue
		}
		if config := channel2advancedCustomConfig[channelId]; config != nil && config.SupportsPath(requestPath) {
			filtered = append(filtered, channelId)
		}
	}
	return filtered
}

func CacheGetChannel(id int) (*Channel, error) {
	if !common.MemoryCacheEnabled {
		return GetChannelById(id, true)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return c, nil
}

func CacheGetChannelInfo(id int) (*ChannelInfo, error) {
	if !common.MemoryCacheEnabled {
		channel, err := GetChannelById(id, true)
		if err != nil {
			return nil, err
		}
		return &channel.ChannelInfo, nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return &c.ChannelInfo, nil
}

func CacheUpdateChannelStatus(id int, status int) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel, ok := channelsIDM[id]; ok {
		channel.Status = status
	}
	if status != common.ChannelStatusEnabled {
		// delete the channel from group2model2channels
		for group, model2channels := range group2model2channels {
			for model, channels := range model2channels {
				for i, channelId := range channels {
					if channelId == id {
						// remove the channel from the slice
						group2model2channels[group][model] = append(channels[:i], channels[i+1:]...)
						break
					}
				}
			}
		}
	}
}

func CacheUpdateChannel(channel *Channel) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel == nil {
		return
	}

	if channelsIDM == nil {
		channelsIDM = make(map[int]*Channel)
	}
	if oldChannel, ok := channelsIDM[channel.Id]; ok {
		logger.LogDebug(nil, "CacheUpdateChannel before: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, oldChannel.ChannelInfo.MultiKeyPollingIndex)
	}
	channelsIDM[channel.Id] = channel
	InvalidateFallbackCandidateCache(channel.Id)
	logger.LogDebug(nil, "CacheUpdateChannel after: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, channel.ChannelInfo.MultiKeyPollingIndex)
}

// GetActiveChannelIDs returns all enabled (active) channel IDs.
func GetActiveChannelIDs() []int {
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()
	var ids []int
	for id, ch := range channelsIDM {
		if ch.Status == common.ChannelStatusEnabled {
			ids = append(ids, id)
		}
	}
	return ids
}

// getChannelsForGroup returns all channel IDs for a group.
func getChannelsForGroup(group string) []int {
	ids := make(map[int]struct{})
	for _, modelChannels := range group2model2channels[group] {
		for _, chID := range modelChannels {
			ids[chID] = struct{}{}
		}
	}
	result := make([]int, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	return result
}

func unionChannelIDs(a, b []int) []int {
	seen := make(map[int]struct{}, len(a)+len(b))
	for _, id := range a {
		seen[id] = struct{}{}
	}
	for _, id := range b {
		seen[id] = struct{}{}
	}
	result := make([]int, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	return result
}

// GetChannelsForGroupModelWithFallback returns all channel candidates for a group+model,
// including channels that can serve the model via fallback mapping.
func GetChannelsForGroupModelWithFallback(group, requestedModel string) []int {
	direct := group2model2channels[group][requestedModel]
	normalized := group2model2channels[group][ratio_setting.FormatMatchingModelName(requestedModel)]
	allDirect := unionChannelIDs(direct, normalized)

	allChannelIDs := getChannelsForGroup(group)
	var extended []int
	for _, chID := range allChannelIDs {
		ch, ok := channelsIDM[chID]
		if !ok {
			continue
		}
		candidates := ch.GetFallbackCandidates(requestedModel)
		for _, candidate := range candidates {
			if channelListContains(group2model2channels[group][candidate], chID) {
				extended = append(extended, chID)
				break
			}
		}
	}
	return unionChannelIDs(allDirect, extended)
}

func channelListContains(list []int, chID int) bool {
	for _, id := range list {
		if id == chID {
			return true
		}
	}
	return false
}

func filterExcluded(ids []int, excluded map[int]struct{}) []int {
	if len(excluded) == 0 {
		return ids
	}
	var result []int
	for _, id := range ids {
		if _, ok := excluded[id]; !ok {
			result = append(result, id)
		}
	}
	return result
}
