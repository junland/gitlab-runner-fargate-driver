package custom

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/aws"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/assertions"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/cli"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/task"
)

type prepareCommandTestCase struct {
	context        context.Context
	fargateConfig  config.Fargate
	metadataConfig config.TaskMetadata
	taskARN        string
	containerIP    string

	fargateInitError        error
	fargateRunTaskError     error
	fargateWaitTaskError    error
	fargateContainerIPError error
	fargateStopTaskError    error
	persistDataError        error

	expectedError error
}

func TestNewPrepareCommand(t *testing.T) {
	cmd := NewPrepareCommand()

	assert.NotNil(t, cmd, "Command should be created")
	assert.NotNil(t, cmd.Handler, "Handler should be created")
}

func TestPrepareCommand_CustomExecute(t *testing.T) {
	testContext, _ := context.WithCancel(context.Background())
	testFargateConfig := config.Fargate{
		Cluster:        "cluster",
		Region:         "region",
		Subnet:         "subnet",
		SecurityGroup:  "security-group",
		TaskDefinition: "task-definition",
		EnablePublicIP: true,
	}
	testMetadataConfig := config.TaskMetadata{
		Directory: "directory",
	}
	testTaskARN := "task-arn"
	testContainerIP := "1.2.3.4"

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
			tt.context = testContext
			tt.fargateConfig = testFargateConfig
			tt.metadataConfig = testMetadataConfig
			tt.taskARN = testTaskARN
			tt.containerIP = testContainerIP

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

			err := prepare.CustomExecute(createCliContextForTests(tt))

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
	setExpectationForFargateRunTask(mock, shouldCallRunTask, testParams)

	// Wait Task API will be called if previous calls succeeded
	shouldCallWaitTask := shouldCallRunTask && testParams.fargateRunTaskError == nil
	setExpectationForFargateWaitTask(mock, shouldCallWaitTask, testParams)

	// Get Container API will be called if previous calls succeeded
	shouldCallGetIP := shouldCallWaitTask && testParams.fargateWaitTaskError == nil
	setExpectationForFargateGetIP(mock, shouldCallGetIP, testParams)

	// Stop Task API will be called if any error occurred after the task was started
	shouldCallStopTask := testParams.fargateRunTaskError != nil ||
		testParams.fargateWaitTaskError != nil ||
		testParams.fargateContainerIPError != nil ||
		testParams.persistDataError != nil
	setExpectationForFargateStopTask(mock, shouldCallStopTask, testParams)
}

func setExpectationForFargateRunTask(mockAwsFargate *aws.MockFargate, shouldCall bool, testParams prepareCommandTestCase) {
	if !shouldCall {
		return
	}

	expectedConnectionSettings := aws.ConnectionSettings{
		Cluster:        testParams.fargateConfig.Cluster,
		Subnet:         testParams.fargateConfig.Subnet,
		SecurityGroup:  testParams.fargateConfig.SecurityGroup,
		EnablePublicIP: testParams.fargateConfig.EnablePublicIP,
	}

	mockAwsFargate.On(
		"RunTask",
		testParams.context,
		testParams.fargateConfig.TaskDefinition,
		expectedConnectionSettings,
	).
		Return(testParams.taskARN, testParams.fargateRunTaskError).
		Once()
}

func setExpectationForFargateWaitTask(mockAwsFargate *aws.MockFargate, shouldCall bool, testParams prepareCommandTestCase) {
	if !shouldCall {
		return
	}

	mockAwsFargate.On(
		"WaitUntilTaskRunning",
		testParams.context,
		testParams.taskARN,
		testParams.fargateConfig.Cluster,
	).
		Return(testParams.fargateWaitTaskError).
		Once()
}

func setExpectationForFargateGetIP(mockAwsFargate *aws.MockFargate, shouldCall bool, testParams prepareCommandTestCase) {
	if !shouldCall {
		return
	}

	mockAwsFargate.On("GetContainerIP",
		testParams.context,
		testParams.taskARN,
		testParams.fargateConfig.Cluster,
		testParams.fargateConfig.EnablePublicIP,
	).
		Return(testParams.containerIP, testParams.fargateContainerIPError).
		Once()
}

func setExpectationForFargateStopTask(mockAwsFargate *aws.MockFargate, shouldCall bool, testParams prepareCommandTestCase) {
	if !shouldCall {
		return
	}

	mockAwsFargate.On(
		"StopTask",
		testParams.context,
		testParams.taskARN,
		testParams.fargateConfig.Cluster,
	).
		Return(testParams.fargateStopTaskError).
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

	expectedData := task.Data{
		TaskARN:     testParams.taskARN,
		ContainerIP: testParams.containerIP,
	}

	mockManager.On("Persist", expectedData).
		Return(testParams.persistDataError).
		Once()
}

func createCliContextForTests(testParams prepareCommandTestCase) *cli.Context {
	cliCtx := new(cli.Context)
	cliCtx.SetConfig(config.Global{
		Fargate:      testParams.fargateConfig,
		TaskMetadata: testParams.metadataConfig,
	})
	cliCtx.SetLogger(createTestLogger())
	cliCtx.Ctx = testParams.context

	return cliCtx
}

func createTestLogger() logging.Logger {
	return test.NewNullLogger()
}
