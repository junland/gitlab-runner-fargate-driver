package custom

import (
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	urfaveCli "github.com/urfave/cli"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/aws"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/assertions"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/cli"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/runner"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/task"
)

func TestNewPrepareCommand(t *testing.T) {
	cmd := NewPrepareCommand()

	assert.NotNil(t, cmd, "Command should be created")
	assert.NotNil(t, cmd.Handler, "Handler should be created")
}

func TestCustomExecutePrepare(t *testing.T) {
	testError := errors.New("simulated error")
	testErrorStopTask := errors.New("simulated error 2")

	runner.InitAdapter()

	tests := map[string]struct {
		fargateInitError        error
		fargateRunTaskError     error
		fargateWaitTaskError    error
		fargateContainerIPError error
		fargateStopTaskError    error
		persistDataError        error
		expectedError           error
	}{
		"Execute prepare with success": {
			fargateInitError:        nil,
			fargateRunTaskError:     nil,
			fargateWaitTaskError:    nil,
			fargateContainerIPError: nil,
			fargateStopTaskError:    nil,
			persistDataError:        nil,
			expectedError:           nil,
		},
		"Error during Fargate Init": {
			fargateInitError:        testError,
			fargateRunTaskError:     nil,
			fargateWaitTaskError:    nil,
			fargateContainerIPError: nil,
			fargateStopTaskError:    nil,
			persistDataError:        nil,
			expectedError:           testError,
		},
		"Error during Fargate Run Task": {
			fargateInitError:        nil,
			fargateRunTaskError:     testError,
			fargateWaitTaskError:    nil,
			fargateContainerIPError: nil,
			fargateStopTaskError:    nil,
			persistDataError:        nil,
			expectedError:           testError,
		},
		"Error during Fargate Wait Task": {
			fargateInitError:        nil,
			fargateRunTaskError:     nil,
			fargateWaitTaskError:    testError,
			fargateContainerIPError: nil,
			fargateStopTaskError:    nil,
			persistDataError:        nil,
			expectedError:           testError,
		},
		"Error during Fargate Wait Task and Stop Task": {
			fargateInitError:        nil,
			fargateRunTaskError:     nil,
			fargateWaitTaskError:    testError,
			fargateContainerIPError: nil,
			fargateStopTaskError:    testErrorStopTask,
			persistDataError:        nil,
			expectedError:           testError,
		},
		"Error during Fargate Get Container IP": {
			fargateInitError:        nil,
			fargateRunTaskError:     nil,
			fargateWaitTaskError:    nil,
			fargateContainerIPError: testError,
			fargateStopTaskError:    nil,
			persistDataError:        nil,
			expectedError:           testError,
		},
		"Error during persisting data": {
			fargateInitError:        nil,
			fargateRunTaskError:     nil,
			fargateWaitTaskError:    nil,
			fargateContainerIPError: nil,
			fargateStopTaskError:    nil,
			persistDataError:        testError,
			expectedError:           testError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			mockAwsFargate := new(aws.MockFargate)
			defer mockAwsFargate.AssertExpectations(t)

			mockMetadataManager := new(task.MockMetadataManager)
			defer mockMetadataManager.AssertExpectations(t)

			setExpectationsForFargate(
				mockAwsFargate,
				tt.fargateInitError,
				tt.fargateRunTaskError,
				tt.fargateWaitTaskError,
				tt.fargateContainerIPError,
				tt.persistDataError,
			)

			setExpectationsForMetadataManager(
				mockMetadataManager,
				tt.fargateInitError,
				tt.fargateRunTaskError,
				tt.fargateWaitTaskError,
				tt.fargateContainerIPError,
				tt.persistDataError,
			)

			prepare := &PrepareCommand{}

			prepare.newFargate = func(logger logging.Logger, awsRegion string) aws.Fargate {
				return mockAwsFargate
			}
			prepare.newMetadataManager = func(logger logging.Logger, directory string) task.MetadataManager {
				return mockMetadataManager
			}

			err := prepare.CustomExecute(createContextForTests())

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err, "Executing the command should not return errors")
			assert.NotEmpty(t, prepare.awsFargate)
			assert.NotEmpty(t, prepare.metadataManager)
		})
	}
}

func setExpectationsForFargate(mock *aws.MockFargate, initError error, runTaskError error,
	waitTaskError error, getIPError error, persistDataError error) {
	setExpectationForFargateInit(mock, true, initError)

	// Run task API will be called only if the Fargate was properly initialized
	shouldCallRunTask := initError == nil
	setExpectationForFargateRunTask(mock, shouldCallRunTask, runTaskError)

	// Wait Task API will be called if previous calls succeeded
	shouldCallWaitTask := shouldCallRunTask && runTaskError == nil
	setExpectationForFargateWaitTask(mock, shouldCallWaitTask, waitTaskError)

	// Get Container API will be called if previous calls succeeded
	shouldCallGetIP := shouldCallWaitTask && waitTaskError == nil
	setExpectationForFargateGetIP(mock, shouldCallGetIP, getIPError)

	// Wait Task API will be called if any error occurred after the task was started
	shouldCallStopTask := runTaskError != nil ||
		waitTaskError != nil || getIPError != nil ||
		persistDataError != nil
	setExpectationForFargateStopTask(mock, shouldCallStopTask, nil)
}

func setExpectationForFargateInit(mockAwsFargate *aws.MockFargate, shouldCall bool, errorToReturn error) {
	if !shouldCall {
		return
	}

	mockAwsFargate.On("Init").
		Return(errorToReturn).
		Once()
}

func setExpectationForFargateRunTask(mockAwsFargate *aws.MockFargate, shouldCall bool, errorToReturn error) {
	if !shouldCall {
		return
	}

	mockAwsFargate.On("RunTask",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("ConnectionSettings"),
	).
		Return("task-arn", errorToReturn).
		Once()
}

func setExpectationForFargateWaitTask(mockAwsFargate *aws.MockFargate, shouldCall bool, errorToReturn error) {
	if !shouldCall {
		return
	}

	mockAwsFargate.On("WaitUntilTaskRunning",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
	).
		Return(errorToReturn).
		Once()
}

func setExpectationForFargateGetIP(mockAwsFargate *aws.MockFargate, shouldCall bool, errorToReturn error) {
	if !shouldCall {
		return
	}

	mockAwsFargate.On("GetContainerIP",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("bool"),
	).
		Return("10.10.10.1", errorToReturn).
		Once()
}

func setExpectationForFargateStopTask(mockAwsFargate *aws.MockFargate, shouldCall bool, errorToReturn error) {
	if !shouldCall {
		return
	}

	mockAwsFargate.On("StopTask",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
	).
		Return(errorToReturn).
		Once()
}

func setExpectationsForMetadataManager(mock *task.MockMetadataManager, initError error, runTaskError error,
	waitTaskError error, getIPError error, persistDataError error) {
	// Data will be persisted if the task is properly in running state and its IP was fetched
	shouldPersistData := initError == nil && runTaskError == nil &&
		waitTaskError == nil && getIPError == nil
	setExpectationForPersistData(mock, shouldPersistData, persistDataError)
}

func setExpectationForPersistData(mockMetadataManager *task.MockMetadataManager, shouldCall bool, errorToReturn error) {
	if !shouldCall {
		return
	}

	mockMetadataManager.On("Persist",
		mock.AnythingOfType("task.Data"),
	).
		Return(errorToReturn).
		Once()
}

func createContextForTests() *cli.Context {
	ctx := &cli.Context{}
	ctx.SetConfig(config.Global{
		FargateConfig: config.FargateConfig{},
	})
	ctx.SetLogger(createTestLogger())
	flagSet := flag.NewFlagSet("myflags", flag.ExitOnError)
	flagSet.String("p", "test", "my p variable")
	flagSet.String("k", "test", "my k variable")
	flagSet.Parse([]string{"p", "k"})
	ctx.Cli = urfaveCli.NewContext(nil, flagSet, nil)
	ctx.Ctx = context.Background()
	return ctx
}

func createTestLogger() logging.Logger {
	return test.NewNullLogger()
}
