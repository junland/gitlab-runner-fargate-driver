package ssh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
	return func(_ string, _ string, _ *ssh.ClientConfig) (client.Client, error) {
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
				return newConnectClientFn(nil, nil), func(_ *testing.T) {}
			},
			expectedError: testErrorSSHInternal,
		},
		"Connect to server error": {
			validPrivateKey: true,
			connectClient: func() (connectClientFn, func(*testing.T)) {
				return newConnectClientFn(nil, testError), func(_ *testing.T) {}
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
				return newConnectClientFn(nil, nil), func(_ *testing.T) {}
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

	_, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	return encodePrivateKeyToPEM(privateKey)
}

func encodePrivateKeyToPEM(privateKey ed25519.PrivateKey) []byte {
	privDER, _ := x509.MarshalPKCS8PrivateKey(privateKey)

	privBlock := pem.Block{
		Type:    "PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	return pem.EncodeToMemory(&privBlock)
}

func TestSSHIntegration(t *testing.T) {
	sshServiceHost := "ssh"
	sshInfoService := fmt.Sprintf("http://%s:8888/", sshServiceHost)

	resp, err := http.Get(sshInfoService)
	if err != nil {
		t.Skipf("Couldn't access SSH service: %v", err)
	}

	defer resp.Body.Close()

	var sshInfo struct {
		Port           int
		Username       string
		HostPrivateKey string
		HostPublicKey  string
		UserPrivateKey string
		UserPublicKey  string
	}

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&sshInfo)
	require.NoError(t, err)

	settings := executors.ConnectionSettings{
		Hostname:   sshServiceHost,
		Port:       sshInfo.Port,
		Username:   sshInfo.Username,
		PrivateKey: []byte(sshInfo.UserPrivateKey),
	}

	tests := map[string]struct {
		ctx          func() context.Context
		assertOutput func(t *testing.T, output string)
	}{
		"context finished before command": {
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			},
			assertOutput: func(t *testing.T, output string) {
				t.Log(output)
				assert.NotContains(t, output, "Exiting!")
			},
		},
		"context finished after command": {
			ctx: func() context.Context {
				return context.Background()
			},
			assertOutput: func(t *testing.T, output string) {
				t.Log(output)
				assert.Contains(t, output, "Exiting!")
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			e, ok := NewExecutor(logging.New()).(*executor)
			require.True(t, ok)

			out := new(strings.Builder)
			e.stdout = out
			e.stderr = out

			script := `
#!/usr/bin/env bash

trap 'echo "Exiting!"' EXIT

for i in $(seq 1 30); do
  echo -n .
  sleep 1
done
`

			err := e.Execute(tt.ctx(), settings, []byte(script))

			assert.NoError(t, err)
			tt.assertOutput(t, out.String())
		})
	}
}

type temporaryError struct{}

func (te *temporaryError) Error() string   { return "this too shall pass" }
func (te *temporaryError) Temporary() bool { return true }
func (te *temporaryError) Timeout() bool   { return false }

type timeoutError struct{}

func (te *timeoutError) Error() string   { return "too late" }
func (te *timeoutError) Temporary() bool { return false }
func (te *timeoutError) Timeout() bool   { return true }

func Test_Retry(t *testing.T) {
	attempts := 0
	someErr := fmt.Errorf("blammo")
	tempErr := temporaryError{}
	toErr := timeoutError{}

	tests := map[string]struct {
		fn           func() error
		wantError    error
		shouldRetry  func(error) bool
		wantAttempts int
	}{
		"immediate success": {
			wantAttempts: 1,
			fn: func() error {
				attempts++
				return nil
			},
		},
		"eventual success": {
			wantAttempts: 3,
			fn: func() error {
				attempts++
				if attempts < 3 {
					return someErr
				}
				return nil
			},
			shouldRetry: func(_ error) bool {
				return true
			},
		},
		"eventual failure": {
			wantAttempts: 5,
			wantError:    someErr,
			fn: func() error {
				attempts++
				return someErr
			},
			shouldRetry: func(_ error) bool {
				return true
			},
		},
		"with non-retryable error": {
			wantAttempts: 1,
			wantError:    someErr,
			fn: func() error {
				attempts++
				return someErr
			},
			shouldRetry: func(_ error) bool {
				return false
			},
		},
		"with temporary error": {
			wantAttempts: 2,
			wantError:    nil,
			fn: func() error {
				attempts++
				if attempts < 2 {
					return &tempErr
				}
				return nil
			},
			shouldRetry: isOpErrorRetryable,
		},
		"with timeout error": {
			wantAttempts: 2,
			wantError:    nil,
			fn: func() error {
				attempts++
				if attempts < 2 {
					return &toErr
				}
				return nil
			},
			shouldRetry: isOpErrorRetryable,
		},

		"with non opError error": {
			wantAttempts: 1,
			wantError:    someErr,
			fn: func() error {
				attempts++
				return someErr
			},
			shouldRetry: isOpErrorRetryable,
		},
	}

	e := executor{}

	for name, tt := range tests {
		attempts = 0
		t.Run(name, func(t *testing.T) {
			err := e.retry(5, 25*time.Millisecond, tt.fn, tt.shouldRetry)

			assert.Equal(t, tt.wantError, err)
			assert.Equal(t, tt.wantAttempts, attempts)
		})
	}
}
