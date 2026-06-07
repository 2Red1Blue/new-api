package service

import (
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func setUpstreamSecretsForTest(t *testing.T, upstream string, crypto string, session string) {
	t.Helper()

	origUpstream := common.UpstreamSecretKey
	origCrypto := common.CryptoSecret
	origSession := common.SessionSecret

	common.UpstreamSecretKey = upstream
	common.CryptoSecret = crypto
	common.SessionSecret = session

	t.Cleanup(func() {
		common.UpstreamSecretKey = origUpstream
		common.CryptoSecret = origCrypto
		common.SessionSecret = origSession
	})
}

func loadUpstreamSecretsFromEnvForTest(t *testing.T) {
	t.Helper()

	upstream := strings.TrimSpace(os.Getenv("UPSTREAM_SECRET_KEY"))
	crypto := strings.TrimSpace(os.Getenv("CRYPTO_SECRET"))
	session := strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	if crypto == "" {
		crypto = session
	}
	setUpstreamSecretsForTest(t, upstream, crypto, session)
}

func TestEncryptDecryptUpstreamPasswordRoundTripUsesUpstreamSecretKey(t *testing.T) {
	setUpstreamSecretsForTest(t, "upstream-secret", "crypto-secret", "session-secret")

	encrypted, err := EncryptUpstreamPassword("secret-value")
	require.NoError(t, err)
	require.NotEmpty(t, encrypted)
	require.NotContains(t, encrypted, "secret-value")

	decrypted, err := DecryptUpstreamPassword(encrypted)
	require.NoError(t, err)
	require.Equal(t, "secret-value", decrypted)
}

func TestEncryptDecryptUpstreamPasswordRoundTripFallsBackToCryptoSecret(t *testing.T) {
	setUpstreamSecretsForTest(t, "", "crypto-secret", "session-secret")

	encrypted, err := EncryptUpstreamPassword("crypto-only-secret")
	require.NoError(t, err)

	decrypted, err := DecryptUpstreamPassword(encrypted)
	require.NoError(t, err)
	require.Equal(t, "crypto-only-secret", decrypted)
}

func TestEncryptDecryptUpstreamPasswordRoundTripFallsBackToSessionSecret(t *testing.T) {
	setUpstreamSecretsForTest(t, "", "", "session-secret")

	encrypted, err := EncryptUpstreamPassword("session-only-secret")
	require.NoError(t, err)

	decrypted, err := DecryptUpstreamPassword(encrypted)
	require.NoError(t, err)
	require.Equal(t, "session-only-secret", decrypted)
}

func TestDecryptUpstreamPasswordFailsWhenSecretChanges(t *testing.T) {
	setUpstreamSecretsForTest(t, "original-secret", "", "")

	encrypted, err := EncryptUpstreamPassword("secret-value")
	require.NoError(t, err)

	common.UpstreamSecretKey = "different-secret"

	decrypted, err := DecryptUpstreamPassword(encrypted)
	require.Error(t, err)
	require.Empty(t, decrypted)
	require.Contains(t, err.Error(), "message authentication failed")
}

func TestEncryptUpstreamPasswordEmptyStringReturnsEmpty(t *testing.T) {
	setUpstreamSecretsForTest(t, "upstream-secret", "", "")

	encrypted, err := EncryptUpstreamPassword("")
	require.NoError(t, err)
	require.Empty(t, encrypted)

	decrypted, err := DecryptUpstreamPassword("")
	require.NoError(t, err)
	require.Empty(t, decrypted)
}

func TestDecryptUpstreamPasswordFixtureFromEnv(t *testing.T) {
	ciphertext := strings.TrimSpace(os.Getenv("UPSTREAM_PASSWORD_TEST_CIPHERTEXT"))
	expectedPlaintext := os.Getenv("UPSTREAM_PASSWORD_TEST_PLAINTEXT")
	if ciphertext == "" || expectedPlaintext == "" {
		t.Skip("set UPSTREAM_PASSWORD_TEST_CIPHERTEXT and UPSTREAM_PASSWORD_TEST_PLAINTEXT to verify a real encrypted password")
	}

	loadUpstreamSecretsFromEnvForTest(t)

	decrypted, err := DecryptUpstreamPassword(ciphertext)
	require.NoError(t, err)
	require.Equal(t, expectedPlaintext, decrypted)
}
