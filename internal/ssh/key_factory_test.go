package ssh

import (
	"crypto/ed25519"
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
	assert.NotNil(t, factory.(*keyFactory).generateEd25519Key)
	assert.NotNil(t, factory.(*keyFactory).newSSHPublicKey)
}

func TestKeyFactory_Create(t *testing.T) {
	testError := errors.New("simulated error")
	testErrorEd25519Internal := new(ErrInvalidPrivateKey)

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	// Create Fake Byte Arrays to Represent ed25519 Key Pairs
	invalidPrivKey := make([]byte, ed25519.PrivateKeySize)
	invalidPubKey := make([]byte, ed25519.PublicKeySize)
	_, _ = rand.Read(invalidPrivKey)
	_, _ = rand.Read(invalidPubKey)

	tests := map[string]struct {
		mockedEd25519PrivKey ed25519.PrivateKey
		mockedEd25519PubKey  ed25519.PublicKey
		ed25519KeyError      error
		publicKeyError       error
		expectedError        error
	}{
		"Key pair generated with success": {
			mockedEd25519PrivKey: privKey,
			mockedEd25519PubKey:  pubKey,
			ed25519KeyError:      nil,
			publicKeyError:       nil,
			expectedError:        nil,
		},
		"Error generating key pair": {
			mockedEd25519PrivKey: nil,
			mockedEd25519PubKey:  nil,
			ed25519KeyError:      testError,
			publicKeyError:       nil,
			expectedError:        testError,
		},
		"Error invalid key pair": {
			mockedEd25519PrivKey: invalidPrivKey,
			mockedEd25519PubKey:  invalidPubKey,
			ed25519KeyError:      nil,
			publicKeyError:       nil,
			expectedError:        testErrorEd25519Internal,
		},
		"Error creating SSH public key": {
			mockedEd25519PrivKey: privKey,
			mockedEd25519PubKey:  pubKey,
			ed25519KeyError:      nil,
			publicKeyError:       testError,
			expectedError:        testError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			factory := NewKeyFactory(test.NewNullLogger())

			factory.(*keyFactory).generateEd25519Key = func(r io.Reader) (ed25519.PublicKey, ed25519.PrivateKey, error) {
				return tt.mockedEd25519PubKey, tt.mockedEd25519PrivKey, tt.ed25519KeyError
			}

			if tt.publicKeyError != nil {
				factory.(*keyFactory).newSSHPublicKey = func(k interface{}) (ssh.PublicKey, error) {
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
