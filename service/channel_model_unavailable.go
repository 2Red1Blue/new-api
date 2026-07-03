package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/samber/hot"
)

const (
	modelUnavailableCacheNamespace = "new-api:channel_model_unavailable:v1"
	modelUnavailableDefaultTTL     = 5 * time.Minute

	ContextKeyBypassedAffinityForModel = "bypassed_affinity_for_model"
	ContextKeyModelFallbackUsed        = "model_fallback_used"
)

var (
	modelUnavailableCacheOnce sync.Once
	modelUnavailableCacheInst *cachex.HybridCache[int]
)

func getModelUnavailableCache() *cachex.HybridCache[int] {
	modelUnavailableCacheOnce.Do(func() {
		capacity := 100_000
		modelUnavailableCacheInst = cachex.NewHybridCache[int](cachex.HybridCacheConfig[int]{
			Namespace: cachex.Namespace(modelUnavailableCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.IntCodec{},
			Memory: func() *hot.HotCache[string, int] {
				return hot.NewHotCache[string, int](hot.LRU, capacity).
					WithTTL(modelUnavailableDefaultTTL).
					WithJanitor().
					Build()
			},
		})
	})
	return modelUnavailableCacheInst
}

func modelUnavailableKey(channelID int, modelName string) string {
	return fmt.Sprintf("%d:%s", channelID, ratio_setting.FormatMatchingModelName(modelName))
}

// MarkModelUnavailableForChannel marks a channel-model pair as unavailable.
func MarkModelUnavailableForChannel(channelID int, modelName string) {
	cache := getModelUnavailableCache()
	key := modelUnavailableKey(channelID, modelName)
	_ = cache.SetWithTTL(key, 1, modelUnavailableDefaultTTL)
}

// ClearModelUnavailableForChannel clears the unavailable mark on success.
func ClearModelUnavailableForChannel(channelID int, modelName string) {
	cache := getModelUnavailableCache()
	_, _ = cache.DeleteMany([]string{modelUnavailableKey(channelID, modelName)})
}

// IsModelUnavailableForChannel checks if a channel-model pair is marked unavailable.
func IsModelUnavailableForChannel(channelID int, modelName string) bool {
	cache := getModelUnavailableCache()
	_, found, _ := cache.Get(modelUnavailableKey(channelID, modelName))
	return found
}

// GetUnavailableChannelIDs returns all channel IDs marked unavailable for a given model.
// Consults HybridCache (Redis+memory with TTL) per active channel — no separate persistent index.
func GetUnavailableChannelIDs(modelName string) map[int]struct{} {
	result := make(map[int]struct{})
	cache := getModelUnavailableCache()
	for _, chID := range model.GetActiveChannelIDs() {
		_, found, _ := cache.Get(modelUnavailableKey(chID, modelName))
		if found {
			result[chID] = struct{}{}
		}
	}
	return result
}

// InvalidateModelUnavailableForChannel clears all negative cache entries for a channel.
func InvalidateModelUnavailableForChannel(channelID int) {
	cache := getModelUnavailableCache()
	prefix := fmt.Sprintf("%d:", channelID)
	keys, _ := cache.Keys()
	var toDelete []string
	for _, k := range keys {
		if strings.HasPrefix(k, prefix) {
			toDelete = append(toDelete, k)
		}
	}
	if len(toDelete) > 0 {
		_, _ = cache.DeleteMany(toDelete)
	}
}
