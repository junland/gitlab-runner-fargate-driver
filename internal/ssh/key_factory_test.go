package ssh

import (
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

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
	testErrorecdsaInternal := new(ErrInvalidPrivateKey)
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tests := map[string]struct {
		mockedecdsaPrivKey ecdsa.PrivateKey
		mockedecdsaPubKey  privKey.Public()
		ecdsaKeyError      error
		publicKeyError       error
		expectedError        error
	}{
		"Key pair generated with success": {
			mockedecdsaPrivKey: privKey,
			mockedecdsaPubKey:  pubKey,
			ecdsaKeyError:      nil,
			publicKeyError:       nil,
			expectedError:        nil,
		},
		"Error generating key pair": {
			mockedecdsaPrivKey: nil,
			mockedecdsaPubKey:  nil,
			ecdsaKeyError:      testError,
			publicKeyError:       nil,
			expectedError:        testError,
		},
		"Error invalid key pair": {
			mockedecdsaPrivKey: invalidPrivKey,
			mockedecdsaPubKey:  invalidPubKey,
			ecdsaKeyError:      nil,
			publicKeyError:       nil,
			expectedError:        testErrorecdsaInternal,
		},
		"Error creating SSH public key": {
			mockedecdsaPrivKey: privKey,
			mockedecdsaPubKey:  pubKey,
			ecdsaKeyError:      nil,
			publicKeyError:       testError,
			expectedError:        testError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			factory := NewKeyFactory(test.NewNullLogger())

			factory.(*keyFactory).generateecdsaKey = func(c elliptic.Curve, _ io.Reader) (*ecdsa.PrivateKey, error) {
				return tt.mockedecdsaPubKey, tt.mockedecdsaPrivKey, tt.ecdsaKeyError
			}

			if tt.publicKeyError != nil {
				factory.(*keyFactory).newSSHPublicKey = func(_ interface{}) (ssh.PublicKey, error) {
					return nil, tt.publicKeyError
				}
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
		})
	}
}
