package service

import "github.com/QuantumNous/new-api/model"

// IsChannelSatisfiableForModel checks if a channel can serve a model request,
// considering direct ability, negative cache, and fallback candidates.
func IsChannelSatisfiableForModel(
	channelID int,
	group, requestedModel string,
	fallbackCandidates []string,
) bool {
	// 1. Direct match in ability table
	if model.IsChannelEnabledForGroupModel(group, requestedModel, channelID) {
		return true
	}
	// 2. Negative cache hit
	if IsModelUnavailableForChannel(channelID, requestedModel) {
		return false
	}
	// 3. Check fallback candidates
	for _, candidate := range fallbackCandidates {
		if model.IsChannelEnabledForGroupModel(group, candidate, channelID) {
			return true
		}
	}
	return false
}
