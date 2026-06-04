package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TestShouldRetryDoesNotTreatClearedRateLimitFlagAsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(channelRateLimitedContextKey, true)
	c.Set(channelRateLimitedContextKey, false)

	err := types.NewErrorWithStatusCode(assertTestError("bad request"), types.ErrorCodeGetChannelFailed, 400)

	if shouldRetry(c, err, 1, false) {
		t.Fatalf("expected cleared rate limit flag not to force retry")
	}
}

func TestShouldExcludeChannelForRequestFailureWaitsForThreshold(t *testing.T) {
	for count := 1; count < perRequestChannelFailureLimit; count++ {
		if shouldExcludeChannelForRequestFailure(count) {
			t.Fatalf("failure count %d should not exclude before threshold %d", count, perRequestChannelFailureLimit)
		}
	}
	if !shouldExcludeChannelForRequestFailure(perRequestChannelFailureLimit) {
		t.Fatalf("failure count %d should exclude at threshold", perRequestChannelFailureLimit)
	}
}

type testError string

func (e testError) Error() string { return string(e) }

func assertTestError(msg string) error { return testError(msg) }
