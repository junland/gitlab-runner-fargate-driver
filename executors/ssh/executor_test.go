package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/executors"
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

func TestExecute(t *testing.T) {
	testError := errors.New("simulated error")
	testErrorSSHInternal := errors.New("ssh: no key found")
	testErrorSession := errors.New("simulated error 1")
	testErrorScript := errors.New("simulated error 2")

	assertErrorByEqual := func(t *testing.T, err error, expectedError error) {
		assertions.ErrorIs(t, err, expectedError)
	}

	// This assert will be necessary because we can't directly compare the
	// ssh.ParsePrivateKey() returned error
	assertErrorByContains := func(t *testing.T, err error, expectedError error) {
		assert.Contains(t, err.Error(), expectedError.Error())
	}

	tests := map[string]struct {
		client                *ssh.Client
		validPrivateKey       bool
		sshHandshakeError     error
		sshCreateSessionError error
		sshExecuteScriptError error
		sshDisconnectError    error
		expectedError         error
		errorAssertion        func(*testing.T, error, error)
	}{
		"Execute with success": {
			client:                &ssh.Client{},
			validPrivateKey:       true,
			sshHandshakeError:     nil,
			sshCreateSessionError: nil,
			sshExecuteScriptError: nil,
			sshDisconnectError:    nil,
			expectedError:         nil,
			errorAssertion:        nil,
		},
		"Connect to server with private key problem": {
			client:                &ssh.Client{},
			validPrivateKey:       false,
			sshHandshakeError:     testError,
			sshCreateSessionError: nil,
			sshExecuteScriptError: nil,
			sshDisconnectError:    nil,
			expectedError:         testErrorSSHInternal,
			errorAssertion:        assertErrorByContains,
		},
		"Connect to server with handshake problem": {
			client:                &ssh.Client{},
			validPrivateKey:       true,
			sshHandshakeError:     testError,
			sshCreateSessionError: nil,
			sshExecuteScriptError: nil,
			sshDisconnectError:    nil,
			expectedError:         testError,
			errorAssertion:        assertErrorByEqual,
		},
		"Connect to server with private key and handshake problems": {
			client:                &ssh.Client{},
			validPrivateKey:       false,
			sshHandshakeError:     nil,
			sshCreateSessionError: nil,
			sshExecuteScriptError: nil,
			sshDisconnectError:    nil,
			expectedError:         testErrorSSHInternal,
			errorAssertion:        assertErrorByContains,
		},
		"Error on creating session": {
			client:                &ssh.Client{},
			validPrivateKey:       true,
			sshHandshakeError:     nil,
			sshCreateSessionError: testErrorSession,
			sshExecuteScriptError: nil,
			sshDisconnectError:    nil,
			expectedError:         testErrorSession,
			errorAssertion:        assertErrorByEqual,
		},
		"Error on executing script": {
			client:                &ssh.Client{},
			validPrivateKey:       true,
			sshHandshakeError:     nil,
			sshCreateSessionError: nil,
			sshExecuteScriptError: testErrorScript,
			sshDisconnectError:    nil,
			expectedError:         testErrorScript,
			errorAssertion:        assertErrorByEqual,
		},
		"Error on creating session and executing script": {
			client:                &ssh.Client{},
			validPrivateKey:       true,
			sshHandshakeError:     nil,
			sshCreateSessionError: testErrorSession,
			sshExecuteScriptError: testErrorScript,
			sshDisconnectError:    nil,
			expectedError:         testErrorSession,
			errorAssertion:        assertErrorByEqual,
		},
		"Error when not connected to server": {
			client:                nil,
			validPrivateKey:       true,
			sshHandshakeError:     nil,
			sshCreateSessionError: nil,
			sshExecuteScriptError: nil,
			sshDisconnectError:    nil,
			expectedError:         ErrNotConnected,
			errorAssertion:        assertErrorByEqual,
		},
		"Error on disconnect from server": {
			client:                &ssh.Client{},
			validPrivateKey:       true,
			sshHandshakeError:     nil,
			sshCreateSessionError: nil,
			sshExecuteScriptError: nil,
			sshDisconnectError:    testError,
			expectedError:         testError,
			errorAssertion:        assertErrorByEqual,
		},
	}

	for tc, tt := range tests {
		t.Run(tc, func(t *testing.T) {
			executor := &executor{logger: createTestLogger()}

			executor.internalDial = func(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
				config.HostKeyCallback("test", nil, nil)
				return tt.client, tt.sshHandshakeError
			}
			executor.internalCreateSession = func(client *ssh.Client) (*ssh.Session, error) {
				return &ssh.Session{}, tt.sshCreateSessionError
			}
			executor.internalExecuteScript = func(session *ssh.Session, script string) error {
				return tt.sshExecuteScriptError
			}
			executor.internalCloseSession = func(session *ssh.Session) {}
			executor.internalDisconnect = func(client *ssh.Client) error {
				return tt.sshDisconnectError
			}

			connection := executors.ConnectionSettings{
				Hostname:   "localhost",
				Port:       22,
				Username:   "root",
				PrivateKey: createFakePrivateKeyForTests(tt.validPrivateKey),
			}

			err := executor.Execute(connection, []byte("echo 1"))

			if tt.expectedError == nil {
				assert.NoError(t, err, "Should not have returned any error")
				assert.Nil(t, executor.client, "SSH client should be nil after disconnecting")
				return
			}

			assert.Error(t, err, "Should have returned error")
			tt.errorAssertion(t, err, tt.expectedError)
		})
	}
}

func createFakePrivateKeyForTests(valid bool) []byte {
	if valid {
		privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
		return encodePrivateKeyToPEM(privateKey)
	}

	return []byte("invalid key")
}

func encodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	privatePEM := pem.EncodeToMemory(&privBlock)

	return privatePEM
}
