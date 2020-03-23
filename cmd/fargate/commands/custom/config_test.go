package custom

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/assertions"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/cli"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/runner"
)

func TestNewConfigCommand(t *testing.T) {
	cmd := NewConfigCommand()

	assert.NotNil(t, cmd, "Command should be created")
	assert.NotNil(t, cmd.Handler, "Handler should be created")
}

func TestCustomExecuteConfig(t *testing.T) {
	initializeAdapterForTesting(t)

	testErrorAdapterInternal := new(errWriteConfig)

	tests := map[string]struct {
		forceWriteConfigProblem bool
		expectedError           error
	}{
		"Execute config with success": {
			forceWriteConfigProblem: false,
			expectedError:           nil,
		},
		"Error on write config": {
			forceWriteConfigProblem: true,
			expectedError:           testErrorAdapterInternal,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			config := new(ConfigCommand)

			if tt.forceWriteConfigProblem {
				originalStdout := os.Stdout
				os.Stdout = nil
				defer func() {
					os.Stdout = originalStdout
				}()
			}

			err := config.CustomExecute(createContextForConfigCmdTests())

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err, "Executing the command should not return errors")
		})
	}
}

func initializeAdapterForTesting(t *testing.T) {
	err := os.Setenv("BUILD_FAILURE_EXIT_CODE", "1")
	require.NoError(t, err)

	err = os.Setenv("SYSTEM_FAILURE_EXIT_CODE", "1")
	require.NoError(t, err)

	err = runner.InitAdapter()
	require.NoError(t, err)
}

func createContextForConfigCmdTests() *cli.Context {
	ctx := new(cli.Context)
	ctx.SetLogger(test.NewNullLogger())
	return ctx
}
