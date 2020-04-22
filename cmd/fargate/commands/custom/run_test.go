package custom

import (
	"errors"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	urfaveCli "github.com/urfave/cli"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/executors"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/assertions"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/cli"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/fs"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/task"
)

type runCommandTestCase struct {
	metadataConfig          config.TaskMetadata
	sshUsername             string
	sshPrivateKey           string
	sshPrivateKeyContent    []byte
	sshPort                 *int
	setCommandLineArguments bool
	scriptPath              string
	scriptContent           []byte
	task                    task.Data

	readScriptError     error
	readPrivateKeyError error
	obtainTaskDataError error
	executeScriptError  error

	expectedError error
}

func TestNewRunCommand(t *testing.T) {
	cmd := NewRunCommand()
	assert.NotNil(t, cmd, "Command should be created")
	assert.NotNil(t, cmd.Handler, "Handler should be created")
}

func TestCustomExecuteRun(t *testing.T) {
	testSSHPort := 1234
	testSSHUsername := "user"
	testSSHPrivateKey := "/home/myuser/.ssh/myprivatekey"
	testSSHPrivateKeyContent := []byte("ssh private key content")
	testMetadataConfig := config.TaskMetadata{
		Directory: "/fargate-driver/",
	}
	testScriptPath := "/path/to/script"
	testScriptContent := []byte("test script")
	testTask := task.Data{
		TaskARN:     "task-arn",
		ContainerIP: "1.2.3.4",
	}
	testError := errors.New("simulated error")

	tests := map[string]runCommandTestCase{
		"Execute run with success": {
			setCommandLineArguments: true,
		},
		"Custom SSH port set": {
			sshPort:                 func(i int) *int { return &i }(testSSHPort),
			setCommandLineArguments: true,
		},
		"Error required parameters not informed": {
			setCommandLineArguments: false,
			expectedError:           ErrMissingRequiredArguments,
		},
		"Error reading script": {
			setCommandLineArguments: true,
			readScriptError:         testError,
			expectedError:           testError,
		},
		"Error reading private key": {
			setCommandLineArguments: true,
			readPrivateKeyError:     testError,
			expectedError:           testError,
		},
		"Error reading task metadata": {
			setCommandLineArguments: true,
			obtainTaskDataError:     testError,
			expectedError:           testError,
		},
		"Error executing script remotely": {
			setCommandLineArguments: true,
			executeScriptError:      testError,
			expectedError:           testError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			tt.sshUsername = testSSHUsername
			tt.sshPrivateKey = testSSHPrivateKey
			tt.sshPrivateKeyContent = testSSHPrivateKeyContent
			tt.metadataConfig = testMetadataConfig
			tt.scriptPath = testScriptPath
			tt.scriptContent = testScriptContent
			tt.task = testTask

			mockMetadataManager := new(task.MockMetadataManager)
			defer mockMetadataManager.AssertExpectations(t)

			mockExecutor := new(executors.MockExecutor)
			defer mockExecutor.AssertExpectations(t)

			mockFS := new(fs.MockFS)
			defer mockFS.AssertExpectations(t)

			// Should call read script if app was properly invoked with required parameters
			shouldCallReadScript := tt.setCommandLineArguments
			setExpectationForReadScriptFile(mockFS, shouldCallReadScript, tt)

			// Should call read private key if no problem was detected reading the script file
			shouldCallReadKey := shouldCallReadScript && tt.readScriptError == nil
			setExpectationForReadKeyFile(mockFS, shouldCallReadKey, tt)

			// Should call metadata manager if no error occurred during reading the script and private key
			shouldCallReadMetadata := shouldCallReadKey && tt.readPrivateKeyError == nil
			setExpectationForReadMetadata(mockMetadataManager, shouldCallReadMetadata, tt)

			// Should call executor if no error occurred on any of the previous calls
			shouldCallExecuteScript := shouldCallReadMetadata && tt.obtainTaskDataError == nil
			setExpectationForExecuteScript(mockExecutor, shouldCallExecuteScript, tt)

			run := new(RunCommand)
			run.newMetadataManager = func(logger logging.Logger, directory string) task.MetadataManager {
				return mockMetadataManager
			}
			run.newExecutor = func(logger logging.Logger) executors.Executor {
				return mockExecutor
			}
			run.newFS = func() fs.FS {
				return mockFS
			}

			err := run.CustomExecute(createContextForRunCmdTests(t, tt))

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err, "Executing the command should not return errors")
			assert.NotEmpty(t, run.metadataManager)
			assert.NotEmpty(t, run.sshExecutor)
			assert.NotEmpty(t, run.fs)
		})
	}
}

func setExpectationForReadScriptFile(mockFS *fs.MockFS, shouldCall bool, testParams runCommandTestCase) {
	if !shouldCall {
		return
	}

	mockFS.On("ReadFile", testParams.scriptPath).
		Return(testParams.scriptContent, testParams.readScriptError).
		Once()
}

func setExpectationForReadKeyFile(mockFS *fs.MockFS, shouldCall bool, testParams runCommandTestCase) {
	if !shouldCall {
		return
	}

	mockFS.On("ReadFile", testParams.sshPrivateKey).
		Return(testParams.sshPrivateKeyContent, testParams.readPrivateKeyError).
		Once()
}

func setExpectationForReadMetadata(mockManager *task.MockMetadataManager, shouldCall bool, testParams runCommandTestCase) {
	if !shouldCall {
		return
	}

	mockManager.On("Get").
		Return(testParams.task, testParams.obtainTaskDataError).
		Once()
}

func setExpectationForExecuteScript(mockExecutor *executors.MockExecutor, shouldCall bool, testParams runCommandTestCase) {
	if !shouldCall {
		return
	}

	expectedConnectionSettings := executors.ConnectionSettings{
		Hostname:   testParams.task.ContainerIP,
		Port:       executors.DefaultPort,
		Username:   testParams.sshUsername,
		PrivateKey: testParams.sshPrivateKeyContent,
	}

	if testParams.sshPort != nil {
		expectedConnectionSettings.Port = *testParams.sshPort
	}

	mockExecutor.On("Execute", expectedConnectionSettings, testParams.scriptContent).
		Return(testParams.executeScriptError).
		Once()
}

func createContextForRunCmdTests(t *testing.T, testParams runCommandTestCase) *cli.Context {
	sshConfig := config.SSH{
		Username:       testParams.sshUsername,
		PrivateKeyPath: testParams.sshPrivateKey,
	}

	if testParams.sshPort != nil {
		sshConfig.Port = *testParams.sshPort
	}

	context := cli.Context{}
	context.SetConfig(config.Global{
		SSH:          sshConfig,
		TaskMetadata: testParams.metadataConfig,
	})
	context.SetLogger(test.NewNullLogger())

	flagSet := flag.NewFlagSet("myflags", flag.ExitOnError)
	if testParams.setCommandLineArguments {
		// Programmatically add the required command line arguments
		err := flagSet.Parse([]string{testParams.scriptPath, "prepare_exec"})
		require.NoError(t, err)
	}

	context.Cli = urfaveCli.NewContext(nil, flagSet, nil)

	return &context
}
