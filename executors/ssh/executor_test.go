package ssh

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/executors"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/executors/ssh/internal/client"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/executors/ssh/internal/session"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/assertions"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
)

func TestNewExecutor(t *testing.T) {
	exec := NewExecutor(createTestLogger())
	assert.NotNil(t, exec, "Function should have instantiated a new executor")
	assert.NotNil(t, exec.(*executor).logger, "Function should have persisted logger")
}

func createTestLogger() logging.Logger {
	return test.NewNullLogger()
}

type connectClientFn func(string, string, *ssh.ClientConfig) (client.Client, error)

func newConnectClientFn(cli client.Client, err error) connectClientFn {
	return func(network string, addr string, config *ssh.ClientConfig) (client.Client, error) {
		return cli, err
	}
}

func TestExecute(t *testing.T) {
	testContext := context.Background()
	testScript := "echo 1"

	testError := errors.New("simulated error")
	testErrorSSHInternal := new(errInvalidPrivateKey)
	testErrorSession := errors.New("simulated error 1")
	testErrorScript := errors.New("simulated error 2")

	tests := map[string]struct {
		client          *ssh.Client
		validPrivateKey bool
		connectClient   func() (connectClientFn, func(*testing.T))
		expectedError   error
	}{
		"Execute with success": {
			validPrivateKey: true,
			connectClient: func() (connectClientFn, func(*testing.T)) {
				sess := new(session.MockSession)
				sess.On("ExecuteScript", testContext, testScript).
					Return(nil).
					Once()
				sess.On("Close").
					Once()

				cli := new(client.MockClient)
				cli.On("NewSession", mock.Anything, mock.Anything).
					Return(sess, nil).
					Once()
				cli.On("Disconnect").
					Return(nil).
					Once()

				mocksAssertions := func(t *testing.T) {
					cli.AssertExpectations(t)
					sess.AssertExpectations(t)
				}

				return newConnectClientFn(cli, nil), mocksAssertions
			},
			expectedError: nil,
		},
		"Private key invalid": {
			validPrivateKey: false,
			connectClient: func() (connectClientFn, func(*testing.T)) {
				return newConnectClientFn(nil, nil), func(t *testing.T) {}
			},
			expectedError: testErrorSSHInternal,
		},
		"Connect to server error": {
			validPrivateKey: true,
			connectClient: func() (connectClientFn, func(*testing.T)) {
				return newConnectClientFn(nil, testError), func(t *testing.T) {}
			},
			expectedError: testError,
		},
		"Error on creating session": {
			validPrivateKey: true,
			connectClient: func() (connectClientFn, func(*testing.T)) {
				cli := new(client.MockClient)
				cli.On("NewSession", mock.Anything, mock.Anything).
					Return(nil, testErrorSession).
					Once()
				cli.On("Disconnect").
					Return(nil).
					Once()

				mocksAssertions := func(t *testing.T) {
					cli.AssertExpectations(t)
				}

				return newConnectClientFn(cli, nil), mocksAssertions
			},
			expectedError: testErrorSession,
		},
		"Error on executing script": {
			validPrivateKey: true,
			connectClient: func() (connectClientFn, func(*testing.T)) {
				sess := new(session.MockSession)
				sess.On("ExecuteScript", testContext, testScript).
					Return(testErrorScript).
					Once()
				sess.On("Close").
					Once()

				cli := new(client.MockClient)
				cli.On("NewSession", mock.Anything, mock.Anything).
					Return(sess, nil).
					Once()
				cli.On("Disconnect").
					Return(nil).
					Once()

				mocksAssertions := func(t *testing.T) {
					cli.AssertExpectations(t)
					sess.AssertExpectations(t)
				}

				return newConnectClientFn(cli, nil), mocksAssertions
			},
			expectedError: testErrorScript,
		},
		"Error when not connected to server": {
			validPrivateKey: true,
			connectClient: func() (connectClientFn, func(*testing.T)) {
				return newConnectClientFn(nil, nil), func(t *testing.T) {}
			},
			expectedError: ErrNotConnected,
		},
		"Error on disconnect from server": {
			validPrivateKey: true,
			connectClient: func() (connectClientFn, func(*testing.T)) {
				sess := new(session.MockSession)
				sess.On("ExecuteScript", testContext, testScript).
					Return(nil).
					Once()
				sess.On("Close").
					Once()

				cli := new(client.MockClient)
				cli.On("NewSession", mock.Anything, mock.Anything).
					Return(sess, nil).
					Once()
				cli.On("Disconnect").
					Return(testError).
					Once()

				mocksAssertions := func(t *testing.T) {
					cli.AssertExpectations(t)
					sess.AssertExpectations(t)
				}

				return newConnectClientFn(cli, nil), mocksAssertions
			},
			expectedError: testError,
		},
		"Error on disconnect from server when script execution also failed": {
			validPrivateKey: true,
			connectClient: func() (connectClientFn, func(*testing.T)) {
				sess := new(session.MockSession)
				sess.On("ExecuteScript", testContext, testScript).
					Return(testErrorScript).
					Once()
				sess.On("Close").
					Once()

				cli := new(client.MockClient)
				cli.On("NewSession", mock.Anything, mock.Anything).
					Return(sess, nil).
					Once()
				cli.On("Disconnect").
					Return(testError).
					Once()

				mocksAssertions := func(t *testing.T) {
					cli.AssertExpectations(t)
					sess.AssertExpectations(t)
				}

				return newConnectClientFn(cli, nil), mocksAssertions
			},
			expectedError: testErrorScript,
		},
	}

	for tc, tt := range tests {
		t.Run(tc, func(t *testing.T) {
			connectClient, assertExpectations := tt.connectClient()
			defer assertExpectations(t)

			executor := &executor{logger: createTestLogger()}
			executor.connectClient = connectClient

			connection := executors.ConnectionSettings{
				Hostname:   "localhost",
				Port:       22,
				Username:   "root",
				PrivateKey: createFakePrivateKeyForTests(tt.validPrivateKey),
			}

			err := executor.Execute(testContext, connection, []byte(testScript))

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err, "Should not have returned any error")
			assert.Nil(t, executor.client, "SSH client should be nil after disconnecting")
		})
	}
}

func createFakePrivateKeyForTests(valid bool) []byte {
	if !valid {
		return []byte("invalid key")
	}

	privateKey, _ := rsa.GenerateKey(rand.Reader, 64)
	return encodePrivateKeyToPEM(privateKey)
}

func encodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	return pem.EncodeToMemory(&privBlock)
}
