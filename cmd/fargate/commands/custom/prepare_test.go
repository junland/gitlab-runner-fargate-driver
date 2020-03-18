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

type prepareCommandTestCase struct {
	fargateInitError        error
	fargateRunTaskError     error
	fargateWaitTaskError    error
	fargateContainerIPError error
	fargateStopTaskError    error
	persistDataError        error
	expectedError           error
}

func TestNewPrepareCommand(t *testing.T) {
	cmd := NewPrepareCommand()

	assert.NotNil(t, cmd, "Command should be created")
	assert.NotNil(t, cmd.Handler, "Handler should be created")
}

func TestCustomExecutePrepare(t *testing.T) {
	testError := errors.New("simulated error")
	testErrorStopTask := errors.New("simulated error 2")

	tests := map[string]prepareCommandTestCase{
		"Execute prepare with success": {},
		"Error during Fargate Init": {
			fargateInitError: testError,
			expectedError:    testError,
		},
		"Error during Fargate Run Task": {
			fargateRunTaskError: testError,
			expectedError:       testError,
		},
		"Error during Fargate Wait Task": {
			fargateWaitTaskError: testError,
			expectedError:        testError,
		},
		"Error during Fargate Wait Task and Stop Task": {
			fargateWaitTaskError: testError,
			fargateStopTaskError: testErrorStopTask,
			expectedError:        testError,
		},
		"Error during Fargate Get Container IP": {
			fargateContainerIPError: testError,
			expectedError:           testError,
		},
		"Error during persisting data": {
			persistDataError: testError,
			expectedError:    testError,
		},
		"Error during persisting data and stop task": {
			persistDataError:     testError,
			fargateStopTaskError: testErrorStopTask,
			expectedError:        testError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			mockAwsFargate := new(aws.MockFargate)
			defer mockAwsFargate.AssertExpectations(t)

			mockMetadataManager := new(task.MockMetadataManager)
			defer mockMetadataManager.AssertExpectations(t)

			setExpectationsForFargate(mockAwsFargate, tt)
			setExpectationsForMetadataManager(mockMetadataManager, tt)

			prepare := new(PrepareCommand)

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

func setExpectationsForFargate(mock *aws.MockFargate, testParams prepareCommandTestCase) {
	mock.On("Init").
		Return(testParams.fargateInitError).
		Once()

	// Run task API will be called only if the Fargate was properly initialized
	shouldCallRunTask := testParams.fargateInitError == nil
	setExpectationForFargateRunTask(mock, shouldCallRunTask, testParams.fargateRunTaskError)

	// Wait Task API will be called if previous calls succeeded
	shouldCallWaitTask := shouldCallRunTask && testParams.fargateRunTaskError == nil
	setExpectationForFargateWaitTask(mock, shouldCallWaitTask, testParams.fargateWaitTaskError)

	// Get Container API will be called if previous calls succeeded
	shouldCallGetIP := shouldCallWaitTask && testParams.fargateWaitTaskError == nil
	setExpectationForFargateGetIP(mock, shouldCallGetIP, testParams.fargateContainerIPError)

	// Wait Task API will be called if any error occurred after the task was started
	shouldCallStopTask := testParams.fargateRunTaskError != nil ||
		testParams.fargateWaitTaskError != nil || testParams.fargateContainerIPError != nil ||
		testParams.persistDataError != nil
	setExpectationForFargateStopTask(mock, shouldCallStopTask, testParams.fargateStopTaskError)
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

func setExpectationsForMetadataManager(mockManager *task.MockMetadataManager, testParams prepareCommandTestCase) {
	// Data will be persisted if the task is properly in running state and its IP was fetched
	shouldPersistData := testParams.fargateInitError == nil &&
		testParams.fargateRunTaskError == nil &&
		testParams.fargateWaitTaskError == nil &&
		testParams.fargateContainerIPError == nil
	if !shouldPersistData {
		return
	}

	mockManager.On("Persist",
		mock.AnythingOfType("task.Data"),
	).
		Return(testParams.persistDataError).
		Once()
}

func createContextForTests() *cli.Context {
	ctx := new(cli.Context)
	ctx.SetConfig(config.Global{
		FargateConfig: config.FargateConfig{},
		TaskMetadata:  config.TaskMetadata{},
	})
	ctx.SetLogger(createTestLogger())
	ctx.Ctx = context.Background()
	return ctx
}

func createTestLogger() logging.Logger {
	return test.NewNullLogger()
}
