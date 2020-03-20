package custom

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/aws"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/assertions"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/cli"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/task"
)

type cleanupCommandTestCase struct {
	fargateInitError     error
	obtainTaskDataError  error
	fargateStopTaskError error
	clearMetadataError   error
	expectedError        error
}

func TestNewCleanupCommand(t *testing.T) {
	cmd := NewCleanupCommand()

	assert.NotNil(t, cmd, "Command should be created")
	assert.NotNil(t, cmd.Handler, "Handler should be created")
}

func TestCustomExecuteCleanup(t *testing.T) {
	testError := errors.New("simulated error")

	tests := map[string]cleanupCommandTestCase{
		"Execute cleanup with success": {},
		"Error during Fargate init": {
			fargateInitError: testError,
			expectedError:    testError,
		},
		"Error reading task metadata": {
			obtainTaskDataError: testError,
			expectedError:       testError,
		},
		"Error stopping Fargate task": {
			fargateStopTaskError: testError,
			expectedError:        testError,
		},
		"Error deleting task metadata": {
			clearMetadataError: testError,
			expectedError:      testError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			mockAwsFargate := new(aws.MockFargate)
			defer mockAwsFargate.AssertExpectations(t)

			mockMetadataManager := new(task.MockMetadataManager)
			defer mockMetadataManager.AssertExpectations(t)

			mockAwsFargate.On("Init").
				Return(tt.fargateInitError).
				Once()

			// Should call get task data if initialization was successful
			shouldCallGetTaskData := tt.fargateInitError == nil
			setExpectationForGetTaskData(mockMetadataManager, shouldCallGetTaskData, tt.obtainTaskDataError)

			// Should call stop task if init and get task data were successful
			shouldCallStopTask := shouldCallGetTaskData && tt.obtainTaskDataError == nil
			setExpectationForStopTask(mockAwsFargate, shouldCallStopTask, tt.fargateStopTaskError)

			// Should call clear metadata if all process worked as expected
			shouldCallClearMetadata := shouldCallStopTask && tt.fargateStopTaskError == nil
			setExpectationForClearMetadata(mockMetadataManager, shouldCallClearMetadata, tt.clearMetadataError)

			cleanup := new(CleanupCommand)
			cleanup.newFargate = func(logger logging.Logger, awsRegion string) aws.Fargate {
				return mockAwsFargate
			}
			cleanup.newMetadataManager = func(logger logging.Logger, directory string) task.MetadataManager {
				return mockMetadataManager
			}

			err := cleanup.CustomExecute(createContextForCleanupCmdTests())

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err, "Executing the command should not return errors")
			assert.NotEmpty(t, cleanup.awsFargate)
			assert.NotEmpty(t, cleanup.metadataManager)
		})
	}
}

func setExpectationForGetTaskData(mockManager *task.MockMetadataManager, shouldCall bool, errorToReturn error) {
	if !shouldCall {
		return
	}

	mockManager.On("Get").
		Return(task.Data{}, errorToReturn).
		Once()
}

func setExpectationForStopTask(mockAwsFargate *aws.MockFargate, shouldCall bool, errorToReturn error) {
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

func setExpectationForClearMetadata(mockManager *task.MockMetadataManager, shouldCall bool, errorToReturn error) {
	if !shouldCall {
		return
	}

	mockManager.On("Clear").
		Return(errorToReturn).
		Once()
}

func createContextForCleanupCmdTests() *cli.Context {
	ctx := new(cli.Context)
	ctx.SetConfig(config.Global{
		FargateConfig: config.FargateConfig{},
		TaskMetadata:  config.TaskMetadata{},
	})
	ctx.SetLogger(test.NewNullLogger())
	ctx.Ctx = context.Background()
	return ctx
}
