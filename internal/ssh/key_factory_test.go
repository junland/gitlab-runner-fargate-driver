package ssh

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/assertions"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
)

func TestNewKeyFactory(t *testing.T) {
	testLogger := test.NewNullLogger()

	factory := NewKeyFactory(testLogger)

	assert.NotNil(t, factory)
	assert.Equal(t, testLogger, factory.(*keyFactory).logger)
	assert.NotNil(t, factory.(*keyFactory).generateECDSAKey)
	assert.NotNil(t, factory.(*keyFactory).newSSHPublicKey)
}

func TestKeyFactory_Create(t *testing.T) {
	testError := errors.New("simulated error")

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tests := map[string]struct {
		mockedECDSAPrivKey *ecdsa.PrivateKey
		ecdsaKeyError      error
		expectedError      error
	}{
		"Key pair generated with success": {
			mockedECDSAPrivKey: privKey,
			ecdsaKeyError:      nil,
			expectedError:      nil,
		},
		"Error generating key pair": {
			mockedECDSAPrivKey: nil,
			ecdsaKeyError:      testError,
			expectedError:      testError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			factory := NewKeyFactory(test.NewNullLogger())

			factory.(*keyFactory).generateECDSAKey = func(_ elliptic.Curve, _ io.Reader) (*ecdsa.PrivateKey, error) {
				return tt.mockedECDSAPrivKey, tt.ecdsaKeyError
			}

			keyPair, err := factory.Create()

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, tt.expectedError)
				assert.Nil(t, keyPair)
				return
			}

			assert.NoError(t, err)
			require.NotNil(t, keyPair)
			assert.NotEmpty(t, keyPair.PublicKey)
			assert.NotEmpty(t, keyPair.PrivateKey)

			// Verify the public key starts with the expected ECDSA prefix
			assert.Contains(t, string(keyPair.PublicKey), "ecdsa-sha2-nistp256")

			// Verify the private key is in PEM format
			assert.Contains(t, string(keyPair.PrivateKey), "-----BEGIN EC PRIVATE KEY-----")
			assert.Contains(t, string(keyPair.PrivateKey), "-----END EC PRIVATE KEY-----")
		})
	}
}

func TestKeyFactory_Create_Integration(t *testing.T) {
	// Integration test that uses the real implementation
	factory := NewKeyFactory(test.NewNullLogger())

	keyPair, err := factory.Create()

	require.NoError(t, err)
	require.NotNil(t, keyPair)
	assert.NotEmpty(t, keyPair.PublicKey)
	assert.NotEmpty(t, keyPair.PrivateKey)

	// Verify the public key is in the expected format
	assert.Contains(t, string(keyPair.PublicKey), "ecdsa-sha2-nistp256")

	// Verify the private key is in PEM format
	assert.Contains(t, string(keyPair.PrivateKey), "-----BEGIN EC PRIVATE KEY-----")
	assert.Contains(t, string(keyPair.PrivateKey), "-----END EC PRIVATE KEY-----")
}
