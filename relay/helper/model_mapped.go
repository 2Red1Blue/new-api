package helper

import (
	"errors"
	"fmt"
	"strings"

	appcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func ModelMappedHelper(c *gin.Context, info *common.RelayInfo, request dto.Request) error {
	if info.ChannelMeta == nil {
		info.ChannelMeta = &common.ChannelMeta{}
	}

	isResponsesCompact := info.RelayMode == relayconstant.RelayModeResponsesCompact
	originModelName := info.OriginModelName
	mappingModelName := originModelName
	if isResponsesCompact && strings.HasSuffix(originModelName, ratio_setting.CompactModelSuffix) {
		mappingModelName = strings.TrimSuffix(originModelName, ratio_setting.CompactModelSuffix)
	}

	// map model name
	modelMapping := c.GetString("model_mapping")
	if modelMapping != "" && modelMapping != "{}" {
		modelMap, err := unmarshalLegacyModelMapping(modelMapping)
		if err != nil {
			return fmt.Errorf("unmarshal_model_mapping_failed")
		}

		// 支持链式模型重定向，最终使用链尾的模型
		currentModel := mappingModelName
		visitedModels := map[string]bool{
			currentModel: true,
		}
		for {
			if mappedModel, exists := modelMap[currentModel]; exists && mappedModel != "" {
				// 模型重定向循环检测，避免无限循环
				if visitedModels[mappedModel] {
					if mappedModel == currentModel {
						if currentModel == info.OriginModelName {
							info.IsModelMapped = false
							return nil
						} else {
							info.IsModelMapped = true
							break
						}
					}
					return errors.New("model_mapping_contains_cycle")
				}
				visitedModels[mappedModel] = true
				currentModel = mappedModel
				info.IsModelMapped = true
			} else {
				break
			}
		}
		if info.IsModelMapped {
			info.UpstreamModelName = currentModel
		}
	}

	// After legacy chained mapping, attempt ordered fallback.
	upstreamModel := resolveFirstAvailableFallback(c, info, mappingModelName)
	if upstreamModel != mappingModelName {
		info.UpstreamModelName = upstreamModel
		info.IsModelMapped = true
		info.PricingModelName = upstreamModel
		if c != nil {
			c.Set(service.ContextKeyModelFallbackUsed, true)
		}
	} else {
		info.PricingModelName = info.OriginModelName
	}

	if isResponsesCompact {
		finalUpstreamModelName := mappingModelName
		if info.IsModelMapped && info.UpstreamModelName != "" {
			finalUpstreamModelName = info.UpstreamModelName
		}
		info.UpstreamModelName = finalUpstreamModelName
		info.OriginModelName = ratio_setting.WithCompactModelSuffix(finalUpstreamModelName)
	}
	if request != nil {
		request.SetModelName(info.UpstreamModelName)
	}
	return nil
}

// resolveFirstAvailableFallback walks the ordered fallback candidates for the current
// channel and returns the first candidate that exists in the channel's ability table.
// It first attempts the legacy chained resolution, then direct fallback candidates.
func resolveFirstAvailableFallback(c *gin.Context, info *common.RelayInfo, requestedModel string) string {
	channelID := c.GetInt("channel_id")
	group := info.TokenGroup
	if group == "auto" {
		group = appcommon.GetContextKeyString(c, constant.ContextKeyAutoGroup)
	}

	// Direct match in ability table — no fallback needed.
	if model.IsChannelEnabledForGroupModel(group, requestedModel, channelID) {
		return requestedModel
	}

	ch, _ := model.CacheGetChannel(channelID)
	if ch == nil {
		return requestedModel
	}

	// First, try legacy chained mapping resolution to its final target.
	chainedFinal := resolveChainedFinal(requestedModel, ch.GetModelMapping())
	if chainedFinal != "" && chainedFinal != requestedModel {
		if model.IsChannelEnabledForGroupModel(group, chainedFinal, channelID) {
			service.ClearModelUnavailableForChannel(channelID, requestedModel)
			return chainedFinal
		}
	}

	// Then, iterate direct fallback candidates.
	for _, candidate := range ch.GetFallbackCandidates(requestedModel) {
		if model.IsChannelEnabledForGroupModel(group, candidate, channelID) {
			service.ClearModelUnavailableForChannel(channelID, requestedModel)
			return candidate
		}
	}

	// All candidates unavailable — write negative cache.
	service.MarkModelUnavailableForChannel(channelID, requestedModel)
	return requestedModel
}

// resolveChainedFinal follows a legacy model_mapping chain (map[string]string) to its
// final target, with cycle detection.
func resolveChainedFinal(start string, modelMappingJSON string) string {
	if modelMappingJSON == "" || modelMappingJSON == "{}" {
		return ""
	}
	var mapping map[string]string
	if err := appcommon.Unmarshal([]byte(modelMappingJSON), &mapping); err != nil {
		return ""
	}
	visited := map[string]bool{start: true}
	current := start
	for {
		next, ok := mapping[current]
		if !ok || next == "" {
			break
		}
		if visited[next] {
			return "" // cycle detected
		}
		visited[next] = true
		current = next
	}
	if current == start {
		return ""
	}
	return current
}

// unmarshalLegacyModelMapping handles both old map[string]string and new
// map[string]interface{} (with array values) formats. For new format it
// extracts the first string element per key to keep chained mapping working.
func unmarshalLegacyModelMapping(raw string) (map[string]string, error) {
	// Try old format first.
	var m map[string]string
	if err := appcommon.Unmarshal([]byte(raw), &m); err == nil {
		return m, nil
	}
	// Try generic format.
	var rawMap map[string]interface{}
	if err := appcommon.Unmarshal([]byte(raw), &rawMap); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(rawMap))
	for k, v := range rawMap {
		switch val := v.(type) {
		case string:
			result[k] = val
		case []interface{}:
			if len(val) > 0 {
				if s, ok := val[0].(string); ok {
					result[k] = s
				}
			}
		}
	}
	return result, nil
}
