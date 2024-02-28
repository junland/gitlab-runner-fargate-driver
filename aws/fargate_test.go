package aws

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/assertions"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
)

//nolint:gocyclo
func TestRunTask(t *testing.T) {
	testContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	testError := errors.New("simulated error")
	testFailure := "simulated failure"
	taskARN := "my-task-arn"
	logger := createTestLogger()
	testEnvVar := map[string]string{"MY_ENV_VAR": "testing"}
	cluster := "cluster-name"
	taskDefinition := "task-def"

	tests := map[string]struct {
		initializeAdapter  bool
		environmentVars    map[string]string
		platformVersion    string
		awsError           error
		expectedARN        string
		expectedError      error
		expectedRunFailure string
		propagateTags      bool
		taskDefinition     string
		clusterName        string
	}{
		"Fargate API returning success": {
			initializeAdapter:  true,
			expectedARN:        taskARN,
			expectedRunFailure: "",
			clusterName:        cluster,
			taskDefinition:     taskDefinition,
		},
		"Fargate API returning success when propagating tags": {
			initializeAdapter:  true,
			expectedARN:        taskARN,
			expectedRunFailure: "",
			propagateTags:      true,
			clusterName:        cluster,
			taskDefinition:     taskDefinition,
		},
		"Fargate API returning success when overriding env variables": {
			initializeAdapter: true,
			environmentVars:   testEnvVar,
			expectedARN:       taskARN,
			clusterName:       cluster,
			taskDefinition:    taskDefinition,
		},
		"Fargate API returning success overriding platform version": {
			initializeAdapter: true,
			platformVersion:   "1.4.0",
			expectedARN:       taskARN,
			clusterName:       cluster,
			taskDefinition:    taskDefinition,
		},
		"Fargate API returning error": {
			initializeAdapter: true,
			awsError:          testError,
			expectedError:     testError,
			clusterName:       cluster,
			taskDefinition:    taskDefinition,
		},
		"Fargate API returning no error but failures running task": {
			initializeAdapter:  true,
			platformVersion:    "1.4.0",
			expectedError:      ErrTaskRunFailure,
			expectedRunFailure: testFailure,
			clusterName:        cluster,
			taskDefinition:     taskDefinition,
		},
		"Fargate adapter not initialized error": {
			expectedError:  ErrNotInitialized,
			clusterName:    cluster,
			taskDefinition: taskDefinition,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			tt := tt
			mockECS := new(mockEcsClient)
			defer mockECS.AssertExpectations(t)

			mockEC2 := new(mockEc2Client)
			defer mockEC2.AssertExpectations(t)

			fargate := NewFargate(logger, "us-east-1", "", "")

			subnet := "subnet-name"
			securityGroup := "security-group-name"

			connectionSettings := ConnectionSettings{
				Subnets:        []*string{&subnet},
				SecurityGroups: []*string{&securityGroup},
				EnablePublicIP: true,
				UsePublicIP:    true,
			}
			taskSettings := TaskSettings{
				Cluster:              tt.clusterName,
				TaskDefinition:       tt.taskDefinition,
				PlatformVersion:      tt.platformVersion,
				EnvironmentVariables: tt.environmentVars,
				PropagateTags:        tt.propagateTags,
			}

			if tt.initializeAdapter {
				output := &ecs.RunTaskOutput{}

				if tt.expectedRunFailure == "" {
					output.Tasks = []*ecs.Task{{TaskArn: &taskARN}}
				} else {
					expectedRunFailure := tt.expectedRunFailure
					output.Failures = []*ecs.Failure{{Reason: &expectedRunFailure}}
				}

				if tt.clusterName != "" && tt.taskDefinition != "" {
					mockECS.On(
						"RunTaskWithContext",
						testContext,
						mock.MatchedBy(func(rti *ecs.RunTaskInput) bool {
							overrides := (fargate.(*awsFargate)).processEnvVariablesToInject(tt.environmentVars)
							return *rti.Cluster == "cluster-name" &&
								*rti.TaskDefinition == "task-def" &&
								((tt.propagateTags && *rti.PropagateTags == "TASK_DEFINITION") || rti.PropagateTags == nil) &&
								assert.Equal(t, rti.Overrides, overrides, "%v", overrides) &&
								((tt.platformVersion != "" && *rti.PlatformVersion == tt.platformVersion) || rti.PlatformVersion == nil) &&
								*rti.NetworkConfiguration.AwsvpcConfiguration.AssignPublicIp == "ENABLED" &&
								*rti.NetworkConfiguration.AwsvpcConfiguration.Subnets[0] == "subnet-name" &&
								*rti.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups[0] == "security-group-name"
						}),
					).
						Return(output, tt.awsError).
						Once()
				}

				err := fargate.Init()
				require.NoError(t, err)

				// Overwrite initialized values with the mocks
				fargate.(*awsFargate).ecsSvc = mockECS
				fargate.(*awsFargate).ec2Svc = mockEC2
			}

			arn, err := fargate.RunTask(testContext, taskSettings, connectionSettings)

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedARN, arn, "Wrong task ARN received")
		})
	}
}

func createTestLogger() logging.Logger {
	return test.NewNullLogger()
}

func TestNewFargate(t *testing.T) {
	logger := createTestLogger()
	awsRegion := "us-east-1"
	f := NewFargate(logger, awsRegion, "", "")
	assert.NotNil(t, f, "Should instantiate the AWS client")
	assert.Equal(t, logger, f.(*awsFargate).logger, "Should have initialized the logger")
	assert.Equal(t, awsRegion, f.(*awsFargate).awsRegion, "Should have initialized the region")
}

func TestInit(t *testing.T) {
	logger := createTestLogger()
	testError := errors.New("simulated error")

	tests := map[string]struct {
		sessionCreator func(awsRegion string) (*session.Session, error)
		roleARN        string
		expectedError  error
	}{
		"Init succeeded": {
			sessionCreator: func(awsRegion string) (*session.Session, error) {
				return session.NewSession(
					&aws.Config{
						Region: aws.String(awsRegion),
					},
				)
			},
			expectedError: nil,
		},
		"Init with role to assume succeeded": {
			sessionCreator: func(awsRegion string) (*session.Session, error) {
				return session.NewSession(
					&aws.Config{
						Region: aws.String(awsRegion),
					},
				)
			},
			roleARN:       "arn:aws:iam::xxxxxxxxx:role/role-name",
			expectedError: nil,
		},
		"Init failed due to AWS session error": {
			sessionCreator: func(_ string) (*session.Session, error) {
				return nil, testError
			},
			expectedError: testError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			awsFargate := new(awsFargate)

			awsFargate.sessionCreator = tt.sessionCreator
			awsFargate.logger = logger
			awsFargate.roleARN = tt.roleARN

			err := awsFargate.Init()

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, testError)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, awsFargate.ecsSvc, "The AWS client should have been initialized")
		})
	}
}

func TestWaitUntilTaskRunning(t *testing.T) {
	testError := errors.New("simulated error")
	waitAttemptsExceedError := errors.New("ResourceNotReady: exceeded wait attempts")
	logger := createTestLogger()

	tests := map[string]struct {
		initializeAdapter bool
		awsError          error
		expectedError     error
	}{
		"Fargate API returning success": {
			initializeAdapter: true,
			awsError:          nil,
			expectedError:     nil,
		},
		"Fargate API returning error": {
			initializeAdapter: true,
			awsError:          testError,
			expectedError:     testError,
		},
		"Fargate adapter not initialized error": {
			initializeAdapter: false,
			awsError:          nil,
			expectedError:     ErrNotInitialized,
		},
		"ECS Task exceeded wait attempts": {
			initializeAdapter: true,
			awsError:          waitAttemptsExceedError,
			expectedError:     waitAttemptsExceedError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			mockECS := new(mockEcsClient)
			defer mockECS.AssertExpectations(t)

			mockEC2 := new(mockEc2Client)
			defer mockEC2.AssertExpectations(t)

			fargate := NewFargate(logger, "us-east-1", "", "")

			if tt.initializeAdapter {
				mockECS.On(
					"WaitUntilTasksRunningWithContext",
					mock.AnythingOfType("backgroundCtx"),
					mock.AnythingOfType("*ecs.DescribeTasksInput"),
					mock.AnythingOfType("WaiterOption"),
				).
					Return(tt.awsError).
					Once()
				mockECS.On(
					"WaitUntilTasksHealthy",
					mock.AnythingOfType("backgroundCtx"),
					mock.AnythingOfType("*ecs.DescribeTasksInput"),
					mock.AnythingOfType("WaiterOption"),
				).
					Return(tt.awsError).
					Maybe()

				err := fargate.Init()
				require.NoError(t, err)

				// Overwrite initialized values with the mocks
				fargate.(*awsFargate).ecsSvc = mockECS
				fargate.(*awsFargate).ec2Svc = mockEC2
			}

			waitConfig := config.TaskWaitConfig{
				MaxAttempts: 100,
				WaitTimeout: 6,
			}
			err := fargate.WaitUntilTaskRunning(context.Background(), "param1", "param2", waitConfig)

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestWaitUntilTaskHealthy(t *testing.T) {
	logger := createTestLogger()
	taskUnhealthyError := errors.New("Task unhealthy")
	getTaskDetailsError := errors.New("Get task details error")
	getTaskDefinitionDetailsError := errors.New("Get task definition details error")
	tests := map[string]struct {
		hasHealthCheck                       bool
		getTaskDetailsError                  error
		getTaskDefinitionDetailsError        error
		waitUntilTaskHealthyWithContextError error
		expectedError                        error
	}{
		"Waits until task healthy": {
			hasHealthCheck:                       true,
			getTaskDetailsError:                  nil,
			getTaskDefinitionDetailsError:        nil,
			waitUntilTaskHealthyWithContextError: nil,
			expectedError:                        nil,
		},
		"Skipped when no healthcheck defined": {
			hasHealthCheck:                       false,
			getTaskDetailsError:                  nil,
			getTaskDefinitionDetailsError:        nil,
			waitUntilTaskHealthyWithContextError: nil,
			expectedError:                        nil,
		},
		"Error when failing to get task details": {
			hasHealthCheck:                       false,
			getTaskDetailsError:                  getTaskDetailsError,
			getTaskDefinitionDetailsError:        nil,
			waitUntilTaskHealthyWithContextError: nil,
			expectedError:                        getTaskDetailsError,
		},
		"Error when failing to get task definition details": {
			hasHealthCheck:                       false,
			getTaskDetailsError:                  nil,
			getTaskDefinitionDetailsError:        getTaskDefinitionDetailsError,
			waitUntilTaskHealthyWithContextError: nil,
			expectedError:                        getTaskDefinitionDetailsError,
		},
		"Error when task not healthy": {
			hasHealthCheck:                       true,
			getTaskDetailsError:                  nil,
			getTaskDefinitionDetailsError:        nil,
			waitUntilTaskHealthyWithContextError: taskUnhealthyError,
			expectedError:                        taskUnhealthyError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			mockECS := new(mockEcsClient)
			defer mockECS.AssertExpectations(t)

			mockEC2 := new(mockEc2Client)
			defer mockEC2.AssertExpectations(t)

			fargate := NewFargate(logger, "us-east-1", "", "")

			mockECS.On(
				"DescribeTasksWithContext",
				mock.AnythingOfType("backgroundCtx"),
				mock.AnythingOfType("*ecs.DescribeTasksInput"),
			).
				Return(&ecs.DescribeTasksOutput{
					Tasks: []*ecs.Task{{TaskDefinitionArn: new(string)}},
				}, tt.getTaskDetailsError).
				Once()

			var containerDefinition *ecs.ContainerDefinition
			if tt.hasHealthCheck {
				mockECS.On(
					"WaitUntilTaskHealthyWithContext",
					mock.AnythingOfType("*context.timerCtx"),
					mock.AnythingOfType("*ecs.DescribeTasksInput"),
					mock.AnythingOfType("request.WaiterOption"),
				).
					Return(tt.waitUntilTaskHealthyWithContextError).
					Once()
				containerDefinition = &ecs.ContainerDefinition{HealthCheck: &ecs.HealthCheck{}}
			} else {
				containerDefinition = &ecs.ContainerDefinition{}
			}

			mockECS.On(
				"DescribeTaskDefinitionWithContext",
				mock.AnythingOfType("backgroundCtx"),
				mock.AnythingOfType("*ecs.DescribeTaskDefinitionInput"),
			).
				Return(&ecs.DescribeTaskDefinitionOutput{
					TaskDefinition: &ecs.TaskDefinition{
						TaskDefinitionArn:    new(string),
						ContainerDefinitions: []*ecs.ContainerDefinition{containerDefinition},
					},
				}, tt.getTaskDefinitionDetailsError).
				Maybe()

			err := fargate.Init()
			require.NoError(t, err)

			// Overwrite initialized values with the mocks
			fargate.(*awsFargate).ecsSvc = mockECS
			fargate.(*awsFargate).ec2Svc = mockEC2

			waitConfig := config.TaskHealthWaitConfig{
				Retries:       3,
				CheckInterval: 30 * time.Second,
			}

			err = fargate.WaitUntilTaskHealthy(context.Background(), "param1", "param2", waitConfig)

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestStopTask(t *testing.T) {
	testError := errors.New("simulated error")
	logger := createTestLogger()

	tests := map[string]struct {
		initializeAdapter bool
		awsTaskOutput     *ecs.StopTaskOutput
		awsError          error
		expectedError     error
		stopReason        string
	}{
		"Fargate API returning success": {
			initializeAdapter: true,
			awsTaskOutput:     &ecs.StopTaskOutput{},
			awsError:          nil,
			expectedError:     nil,
		},
		"Fargate API returning success with stop reason": {
			initializeAdapter: true,
			awsTaskOutput:     &ecs.StopTaskOutput{},
			awsError:          nil,
			expectedError:     nil,
			stopReason:        "Task stopped by user",
		},
		"Fargate API returning error and task output": {
			initializeAdapter: true,
			awsTaskOutput:     &ecs.StopTaskOutput{},
			awsError:          testError,
			expectedError:     testError,
			stopReason:        "",
		},
		"Fargate API returning error without task output": {
			initializeAdapter: true,
			awsTaskOutput:     nil,
			awsError:          testError,
			expectedError:     testError,
			stopReason:        "",
		},
		"Fargate adapter not initialized error": {
			initializeAdapter: false,
			awsTaskOutput:     nil,
			awsError:          nil,
			expectedError:     ErrNotInitialized,
			stopReason:        "",
		},
	}

	for tn, tt := range tests {
		tt := tt
		t.Run(tn, func(t *testing.T) {
			mockECS := new(mockEcsClient)
			defer mockECS.AssertExpectations(t)

			mockEC2 := new(mockEc2Client)
			defer mockEC2.AssertExpectations(t)

			fargate := NewFargate(logger, "us-east-1", "", "")

			clusterName := "ecs-cluster-name"
			taskArn := "arn:task:mock"
			if tt.initializeAdapter {
				stopReason := tt.stopReason
				mockECS.On(
					"StopTaskWithContext",
					mock.AnythingOfType("backgroundCtx"),
					mock.MatchedBy(func(sti *ecs.StopTaskInput) bool {
						return *sti.Cluster == clusterName &&
							*sti.Task == taskArn &&
							assert.Equal(t, &stopReason, sti.Reason)
					}),
				).
					Return(tt.awsTaskOutput, tt.awsError).
					Once()

				err := fargate.Init()
				require.NoError(t, err)

				// Overwrite initialized values with the mocks
				fargate.(*awsFargate).ecsSvc = mockECS
				fargate.(*awsFargate).ec2Svc = mockEC2
			}

			err := fargate.StopTask(context.Background(), taskArn, clusterName, tt.stopReason)

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestGetContainerIP(t *testing.T) {
	testError := errors.New("simulated error")
	logger := createTestLogger()

	tests := map[string]struct {
		initializeAdapter  bool
		enablePublicIP     bool
		usePublicIP        bool
		awsDescribeTask    error
		awsDescribeNetwork error
		expectedIP         string
		expectedError      error
	}{
		"Successfully obtained IP when private desired": {
			initializeAdapter: true,
			expectedIP:        "10.0.0.1",
		},
		"Successfully obtained IP when public desired": {
			initializeAdapter: true,
			enablePublicIP:    true,
			usePublicIP:       true,
			expectedIP:        "172.0.0.1",
		},
		"Successfully obtained IP when private desired and public enabled": {
			initializeAdapter: true,
			enablePublicIP:    true,
			expectedIP:        "10.0.0.1",
		},
		"Successfully obtained IP when public disabled": {
			initializeAdapter: true,
			expectedIP:        "10.0.0.1",
		},
		"Error during fetching task info for private IP": {
			initializeAdapter: true,
			awsDescribeTask:   testError,
			expectedIP:        "",
			expectedError:     testError,
		},
		"Error during fetching task info for public IP": {
			initializeAdapter: true,
			enablePublicIP:    true,
			usePublicIP:       true,
			awsDescribeTask:   testError,
			expectedIP:        "",
			expectedError:     testError,
		},
		"Error during fetching ip for public IP": {
			initializeAdapter:  true,
			enablePublicIP:     true,
			usePublicIP:        true,
			awsDescribeNetwork: testError,
			expectedIP:         "",
			expectedError:      testError,
		},
		"Fargate adapter not initialized error": {
			enablePublicIP: true,
			expectedIP:     "",
			expectedError:  ErrNotInitialized,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			mockECS := mockFargateDescribeTask(tt.initializeAdapter, tt.awsDescribeTask)
			defer mockECS.AssertExpectations(t)

			networkAPIShouldBeCalled := tt.enablePublicIP &&
				tt.awsDescribeTask == nil &&
				tt.initializeAdapter && tt.usePublicIP
			mockEC2 := mockFargateDescribeNetworkInterfaces(
				networkAPIShouldBeCalled, tt.awsDescribeNetwork,
			)
			defer mockEC2.AssertExpectations(t)

			fargate := NewFargate(logger, "us-east-1", "", "")
			if tt.initializeAdapter {
				err := fargate.Init()
				require.NoError(t, err)

				// Overwrite initialized values with the mocks
				fargate.(*awsFargate).ecsSvc = mockECS
				fargate.(*awsFargate).ec2Svc = mockEC2
			}

			ip, err := fargate.GetContainerIP(context.Background(), "param1", "param2", tt.enablePublicIP, tt.usePublicIP)
			assert.Equal(t, tt.expectedIP, ip)

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func mockFargateDescribeNetworkInterfaces(shouldBeCalled bool, errorToReturn error) *mockEc2Client {
	mockEC2 := new(mockEc2Client)

	if !shouldBeCalled {
		return mockEC2
	}

	publicIP := "172.0.0.1"
	mockEC2.On(
		"DescribeNetworkInterfacesWithContext",
		mock.AnythingOfType("backgroundCtx"),
		mock.AnythingOfType("*ec2.DescribeNetworkInterfacesInput"),
	).
		Return(
			&ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []*ec2.NetworkInterface{
					{
						Association: &ec2.NetworkInterfaceAssociation{
							PublicIp: &publicIP,
						},
					},
				},
			}, errorToReturn,
		).
		Once()

	return mockEC2
}

func mockFargateDescribeTask(shouldBeCalled bool, errorToReturn error) *mockEcsClient {
	mockECS := new(mockEcsClient)

	if !shouldBeCalled {
		return mockECS
	}

	privateIP := "10.0.0.1"
	expectedNetworkIDKey := "networkInterfaceId"
	expectedNetworkIDValue := "net-id"
	mockECS.On(
		"DescribeTasksWithContext",
		mock.AnythingOfType("backgroundCtx"),
		mock.AnythingOfType("*ecs.DescribeTasksInput"),
	).
		Return(
			&ecs.DescribeTasksOutput{
				Tasks: []*ecs.Task{
					{
						Containers: []*ecs.Container{
							{
								NetworkInterfaces: []*ecs.NetworkInterface{
									{PrivateIpv4Address: &privateIP},
								},
							},
						},
						Attachments: []*ecs.Attachment{
							{
								Details: []*ecs.KeyValuePair{
									{
										Name:  &expectedNetworkIDKey,
										Value: &expectedNetworkIDValue,
									},
								},
							},
						},
					},
				},
			}, errorToReturn,
		).
		Once()

	return mockECS
}

func TestGetSTSTags(t *testing.T) {
	tests := map[string]struct {
		stsVariables       string
		stsVariablesValues map[string]string
		exptectedStsTags   []*sts.Tag
		expectedTagKeys    []*string
	}{
		"Empty tags when no variables": {
			stsVariables:     "",
			exptectedStsTags: []*sts.Tag{},
			expectedTagKeys:  []*string{},
		},
		"Tags for existing env variables": {
			stsVariables: "TEST,TEST_VAR",
			stsVariablesValues: map[string]string{
				"TEST":     "aaa",
				"TEST_VAR": "bbb",
			},
			exptectedStsTags: []*sts.Tag{
				{
					Key:   aws.String("TEST"),
					Value: aws.String("aaa"),
				},
				{
					Key:   aws.String("TEST_VAR"),
					Value: aws.String("bbb"),
				},
			},
			expectedTagKeys: []*string{
				aws.String("TEST"),
				aws.String("TEST_VAR"),
			},
		}, "Tags for missing env variables": {
			stsVariables: "TEST,TEST_VAR",
			stsVariablesValues: map[string]string{
				"TEST": "aaa",
			},
			exptectedStsTags: []*sts.Tag{
				{
					Key:   aws.String("TEST"),
					Value: aws.String("aaa"),
				},
				{
					Key:   aws.String("TEST_VAR"),
					Value: aws.String(""),
				},
			},
			expectedTagKeys: []*string{
				aws.String("TEST"),
				aws.String("TEST_VAR"),
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			for k, v := range tt.stsVariablesValues {
				err := os.Setenv("CUSTOM_ENV_"+k, v)
				require.NoError(t, err)
			}
			stsTags, tagKeys := getSTSTags(tt.stsVariables)
			assert.Equal(t, tt.exptectedStsTags, stsTags)
			assert.Equal(t, tt.expectedTagKeys, tagKeys)
			for k := range tt.stsVariablesValues {
				err := os.Unsetenv("CUSTOM_ENV_" + k)
				require.NoError(t, err)
			}
		})
	}
}
