package aws

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
	mocks "gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/mocks/aws"
)

func TestRunTask(t *testing.T) {
	testError := errors.New("simulated error")

	tests := []struct {
		name                string
		awsReturn           error
		expectedARNReturn   string
		expectedErrorReturn error
	}{
		{
			name:                "Fargate API returning success",
			awsReturn:           nil,
			expectedARNReturn:   "my-task-arn",
			expectedErrorReturn: nil,
		},
		{
			name:                "Fargate API returning error",
			awsReturn:           testError,
			expectedARNReturn:   "",
			expectedErrorReturn: testError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskARN := "my-task-arn"

			mockECS := new(mocks.ECSClient)
			defer mockECS.AssertExpectations(t)

			mockECS.On(
				"RunTaskWithContext",
				mock.AnythingOfType("*context.emptyCtx"),
				mock.AnythingOfType("*ecs.RunTaskInput"),
			).
				Return(
					&ecs.RunTaskOutput{
						Tasks: []*ecs.Task{&ecs.Task{TaskArn: &taskARN}},
					}, tt.awsReturn,
				).
				Once()

			logger := createTestLogger()

			awsFargate := awsFargate{ecsSvc: mockECS, logger: logger}

			arn, err := awsFargate.RunTask(context.Background(), "param1", "param2", "param3", "param4", true)

			if tt.expectedErrorReturn != nil {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedARNReturn, arn, "Wrong task ARN received")
		})
	}
}

func createTestLogger() logging.Logger {
	return test.NewNullLogger()
}

func TestNewFargate(t *testing.T) {
	f := NewFargate()
	assert.NotNil(t, f, "Should instantiate the AWS client")
}

func TestInit(t *testing.T) {
	testError := errors.New("simulated error")

	tests := []struct {
		name                     string
		errorDuringCreateSession bool
		expectedErrorReturn      error
	}{
		{
			name:                     "Init succeeded",
			errorDuringCreateSession: false,
			expectedErrorReturn:      nil,
		},
		{
			name:                     "Init failed",
			errorDuringCreateSession: true,
			expectedErrorReturn:      testError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := createTestLogger()
			awsFargate := awsFargate{}

			if tt.errorDuringCreateSession {
				awsFargate.sessionCreator = func(string) (*session.Session, error) {
					return nil, testError
				}
			} else {
				awsFargate.sessionCreator = func(awsRegion string) (*session.Session, error) {
					return session.NewSession(
						&aws.Config{
							Region: aws.String(awsRegion),
						},
					)
				}
			}

			err := awsFargate.Init(logger, "us-east-1")

			if tt.expectedErrorReturn != nil {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, logger, awsFargate.logger, "The logger should have been initialized")
			assert.NotNil(t, awsFargate.ecsSvc, "The AWS client should have been initialized")
		})
	}
}

func TestWaitUntilTaskRunning(t *testing.T) {
	testError := errors.New("simulated error")

	tests := []struct {
		name           string
		awsReturn      error
		expectedReturn error
	}{
		{
			name:           "Fargate API returning success",
			awsReturn:      nil,
			expectedReturn: nil,
		},
		{
			name:           "Fargate API returning error",
			awsReturn:      testError,
			expectedReturn: testError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockECS := new(mocks.ECSClient)
			defer mockECS.AssertExpectations(t)

			mockECS.On(
				"WaitUntilTasksRunningWithContext",
				mock.AnythingOfType("*context.emptyCtx"),
				mock.AnythingOfType("*ecs.DescribeTasksInput"),
			).
				Return(tt.awsReturn).
				Once()

			logger := createTestLogger()

			awsFargate := awsFargate{ecsSvc: mockECS, logger: logger}

			err := awsFargate.WaitUntilTaskRunning(context.Background(), "param1", "param2")

			if tt.expectedReturn != nil {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestStopTask(t *testing.T) {
	testError := errors.New("simulated error")

	tests := []struct {
		name                string
		awsTaskOutputReturn *ecs.StopTaskOutput
		awsErrorReturn      error
		expectedReturn      error
	}{
		{
			name:                "Fargate API returning success",
			awsTaskOutputReturn: &ecs.StopTaskOutput{},
			awsErrorReturn:      nil,
			expectedReturn:      nil,
		},
		{
			name:                "Fargate API returning error and task output",
			awsTaskOutputReturn: &ecs.StopTaskOutput{},
			awsErrorReturn:      testError,
			expectedReturn:      testError,
		},
		{
			name:                "Fargate API returning error without task output",
			awsTaskOutputReturn: nil,
			awsErrorReturn:      testError,
			expectedReturn:      testError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockECS := new(mocks.ECSClient)
			defer mockECS.AssertExpectations(t)

			mockECS.On(
				"StopTaskWithContext",
				mock.AnythingOfType("*context.emptyCtx"),
				mock.AnythingOfType("*ecs.StopTaskInput"),
			).
				Return(tt.awsTaskOutputReturn, tt.awsErrorReturn).
				Once()

			logger := createTestLogger()

			awsFargate := awsFargate{ecsSvc: mockECS, logger: logger}

			err := awsFargate.StopTask(context.Background(), "param1", "param2")

			if tt.expectedReturn != nil {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestGetContainerIP(t *testing.T) {
	testError := errors.New("simulated error")

	tests := []struct {
		name                     string
		usePublicIP              bool
		awsDescribeTaskReturn    error
		awsDescribeNetworkReturn error
		expectedIP               string
		expectedError            error
	}{
		{
			name:                     "Successfully obtained IP when private desired",
			usePublicIP:              false,
			awsDescribeTaskReturn:    nil,
			awsDescribeNetworkReturn: nil,
			expectedIP:               "10.0.0.1",
			expectedError:            nil,
		},
		{
			name:                     "Successfully obtained IP when public desired",
			usePublicIP:              true,
			awsDescribeTaskReturn:    nil,
			awsDescribeNetworkReturn: nil,
			expectedIP:               "172.0.0.1",
			expectedError:            nil,
		},
		{
			name:                     "Error during fetching task info for private IP",
			usePublicIP:              false,
			awsDescribeTaskReturn:    testError,
			awsDescribeNetworkReturn: nil,
			expectedIP:               "",
			expectedError:            testError,
		},
		{
			name:                     "Error during fetching task info for public IP",
			usePublicIP:              true,
			awsDescribeTaskReturn:    testError,
			awsDescribeNetworkReturn: nil,
			expectedIP:               "",
			expectedError:            testError,
		},
		{
			name:                     "Error during fetching ip for public IP",
			usePublicIP:              true,
			awsDescribeTaskReturn:    nil,
			awsDescribeNetworkReturn: testError,
			expectedIP:               "",
			expectedError:            testError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockECS := mockFargateDescribeTask(tt.awsDescribeTaskReturn)
			defer mockECS.AssertExpectations(t)

			var mockEC2 *mocks.EC2Client
			if tt.usePublicIP && tt.awsDescribeTaskReturn == nil {
				mockEC2 = mockFargateDescribeNetworkInterfaces(tt.awsDescribeNetworkReturn)
				defer mockEC2.AssertExpectations(t)
			}

			logger := createTestLogger()

			awsFargate := awsFargate{ecsSvc: mockECS, ec2Svc: mockEC2, logger: logger}

			ip, err := awsFargate.GetContainerIP(context.Background(), "param1", "param2", tt.usePublicIP)

			assert.Equal(t, tt.expectedIP, ip)

			if tt.expectedError != nil {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func mockFargateDescribeNetworkInterfaces(errorToReturn error) *mocks.EC2Client {
	mockEC2 := new(mocks.EC2Client)

	publicIP := "172.0.0.1"
	mockEC2.On(
		"DescribeNetworkInterfacesWithContext",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("*ec2.DescribeNetworkInterfacesInput"),
	).
		Return(
			&ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []*ec2.NetworkInterface{
					&ec2.NetworkInterface{
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

func mockFargateDescribeTask(errorToReturn error) *mocks.ECSClient {
	mockECS := new(mocks.ECSClient)

	privateIP := "10.0.0.1"
	expectedNetworkIDKey := "networkInterfaceId"
	expectedNetworkIDValue := "net-id"
	mockECS.On(
		"DescribeTasksWithContext",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("*ecs.DescribeTasksInput"),
	).
		Return(
			&ecs.DescribeTasksOutput{
				Tasks: []*ecs.Task{
					&ecs.Task{
						Containers: []*ecs.Container{
							&ecs.Container{
								NetworkInterfaces: []*ecs.NetworkInterface{
									&ecs.NetworkInterface{PrivateIpv4Address: &privateIP},
								},
							},
						},
						Attachments: []*ecs.Attachment{
							&ecs.Attachment{
								Details: []*ecs.KeyValuePair{
									&ecs.KeyValuePair{
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
