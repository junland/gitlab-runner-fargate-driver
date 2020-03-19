package custom

import (
	"errors"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	setCommandLineArguments bool
	readScriptError         error
	readPrivateKeyError     error
	obtainTaskDataError     error
	executeScriptError      error
	expectedError           error
}

func TestNewRunCommand(t *testing.T) {
	cmd := NewRunCommand()
	assert.NotNil(t, cmd, "Command should be created")
	assert.NotNil(t, cmd.Handler, "Handler should be created")
}

func TestCustomExecuteRun(t *testing.T) {
	testError := errors.New("simulated error")

	tests := map[string]runCommandTestCase{
		"Execute run with success": {
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
			mockMetadataManager := new(task.MockMetadataManager)
			defer mockMetadataManager.AssertExpectations(t)

			mockExecutor := new(executors.MockExecutor)
			defer mockExecutor.AssertExpectations(t)

			mockFS := new(fs.MockFS)
			defer mockFS.AssertExpectations(t)

			// Should call read script if app was properly invoked with required parameters
			shouldCallReadScript := tt.setCommandLineArguments
			setExpectationForReadFiles(mockFS, shouldCallReadScript, tt.readScriptError)

			// Should call read private key if no problem was detected reading the script file
			shouldCallReadKey := shouldCallReadScript && tt.readScriptError == nil
			setExpectationForReadFiles(mockFS, shouldCallReadKey, tt.readPrivateKeyError)

			// Should call metadata manager if no error occured during reading the script and private key
			shouldCallReadMetadata := shouldCallReadKey && tt.readPrivateKeyError == nil
			setExpectationForReadMetadata(mockMetadataManager, shouldCallReadMetadata, tt.obtainTaskDataError)

			// Should call executor if no error ocurred on any of the previous calls
			shouldCallExecuteScript := shouldCallReadMetadata && tt.obtainTaskDataError == nil
			setExpectationForExecuteScript(mockExecutor, shouldCallExecuteScript, tt.executeScriptError)

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

			err := run.CustomExecute(createContextForRunCmdTests(tt.setCommandLineArguments))

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

func setExpectationForReadFiles(mockFS *fs.MockFS, shouldCall bool, errorToReturn error) {
	if !shouldCall {
		return
	}

	mockFS.On("ReadFile",
		mock.AnythingOfType("string"),
	).
		Return([]byte("echo 1"), errorToReturn).
		Once()
}

func setExpectationForReadMetadata(mockManager *task.MockMetadataManager, shouldCall bool, errorToReturn error) {
	if !shouldCall {
		return
	}

	mockManager.On("Get").
		Return(task.Data{}, errorToReturn).
		Once()
}

func setExpectationForExecuteScript(mockExecutor *executors.MockExecutor, shouldCall bool, errorToReturn error) {
	if !shouldCall {
		return
	}

	mockExecutor.On("Execute",
		mock.AnythingOfType("executors.ConnectionSettings"),
		mock.AnythingOfType("[]uint8"),
	).
		Return(errorToReturn).
		Once()
}

func createContextForRunCmdTests(setCommandLineArguments bool) *cli.Context {
	context := cli.Context{}
	context.SetConfig(config.Global{
		SSHConfig: config.SSHConfig{
			Username:       "root",
			PrivateKeyPath: "/home/myuser/.ssh/myprivatekey",
		},
		TaskMetadata: config.TaskMetadata{Directory: "/fargate-driver/"},
	})
	context.SetLogger(test.NewNullLogger())
	flagSet := flag.NewFlagSet("myflags", flag.ExitOnError)
	if setCommandLineArguments {
		// Programmatically add the required command line arguments
		flagSet.String("path_to_script", "/home/myuser/script.sh", "a bash script to execute")
		flagSet.String("stage_name", "prepare_exec", "name of the job stage")
		flagSet.Parse([]string{"path_to_script", "stage_name"})
	}
	context.Cli = urfaveCli.NewContext(nil, flagSet, nil)
	return &context
}
