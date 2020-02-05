package aws

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ecs/ecsiface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
)

/*
 * Mock the ec2 client for using during the tests
 */
type MockEC2Client struct {
	ec2iface.EC2API
	mock.Mock
}

func (m *MockEC2Client) DescribeNetworkInterfaces(input *ec2.DescribeNetworkInterfacesInput) (*ec2.DescribeNetworkInterfacesOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*ec2.DescribeNetworkInterfacesOutput), args.Error(1)
}
func createTestLogger() logging.Logger {
	return test.NewNullLogger()
}

/*
 * Mock the ecs client for using during the tests
 */
type MockECSClient struct {
	ecsiface.ECSAPI
	mock.Mock
}

func (m *MockECSClient) RunTask(runTaskInput *ecs.RunTaskInput) (*ecs.RunTaskOutput, error) {
	args := m.Called(runTaskInput)
	return args.Get(0).(*ecs.RunTaskOutput), args.Error(1)
}
func (m *MockECSClient) WaitUntilTasksRunning(input *ecs.DescribeTasksInput) error {
	args := m.Called(input)
	return args.Error(0)
}
func (m *MockECSClient) StopTask(input *ecs.StopTaskInput) (*ecs.StopTaskOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*ecs.StopTaskOutput), args.Error(1)
}
func (m *MockECSClient) DescribeTasks(input *ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*ecs.DescribeTasksOutput), args.Error(1)
}

func TestRunTaskWithSuccess(t *testing.T) {

	expectedTaskArn := "my-task-arn"

	mockEcs := new(MockECSClient)
	mockEcs.On("RunTask", mock.AnythingOfType("*ecs.RunTaskInput")).Return(
		&ecs.RunTaskOutput{
			Tasks: []*ecs.Task{&ecs.Task{TaskArn: &expectedTaskArn}},
		}, nil,
	)

	logger := createTestLogger()

	awsFargate := awsFargate{ecsSvc: mockEcs, logger: logger}

	arn, err := awsFargate.RunTask("param1", "param2", "param3", "param4", true)

	mockEcs.AssertExpectations(t)
	mockEcs.AssertNumberOfCalls(t, "RunTask", 1)

	assert.Nil(t, err, "Should have returned no errors")
	assert.Equal(t, expectedTaskArn, arn, "Wrong task ARN received")
}

func TestRunTaskWithErrorDuringAwsCall(t *testing.T) {

	mockEcs := new(MockECSClient)
	mockEcs.On("RunTask", mock.AnythingOfType("*ecs.RunTaskInput")).Return(
		new(ecs.RunTaskOutput), fmt.Errorf("Simulated error"),
	)

	logger := createTestLogger()

	awsFargate := awsFargate{ecsSvc: mockEcs, logger: logger}

	arn, err := awsFargate.RunTask("param1", "param2", "param3", "param4", false)

	mockEcs.AssertExpectations(t)
	mockEcs.AssertNumberOfCalls(t, "RunTask", 1)

	assert.NotNil(t, err, "Should have returned error")
	assert.Contains(t, err.Error(), "Simulated error", "Did not return the expected error")
	assert.Empty(t, arn, "ARN should be empty")
}

func TestNewFargate(t *testing.T) {
	f := NewFargate()
	assert.NotNil(t, f, "Should instantiate the AWS client")
}

func TestInitWithSuccess(t *testing.T) {

	logger := createTestLogger()
	awsFargate := awsFargate{}

	awsFargate.Init(logger, "us-east-1")

	assert.Equal(t, logger, awsFargate.logger, "The logger should have been initialized")
	assert.NotNil(t, awsFargate.ecsSvc, "The AWS client should have been initialized")
}

func TestInitWithFailureDueToAwsLibInitialization(t *testing.T) {

	logger := createTestLogger()
	awsFargate := awsFargate{}

	sessionCreator = func(string) (*session.Session, error) {
		return nil, fmt.Errorf("Simulated error")
	}
	err := awsFargate.Init(logger, "us-east-1")

	assert.Nil(t, awsFargate.logger, "The logger should have been initialized")
	assert.Nil(t, awsFargate.ecsSvc, "The AWS client should have been initialized")
	assert.NotNil(t, err, "An error should have been returned")
	assert.Contains(t, err.Error(), "Simulated error", "Did not return the expected error")
}

func TestWaitUntilTaskRunningWithSuccess(t *testing.T) {

	mockEcs := new(MockECSClient)
	mockEcs.On("WaitUntilTasksRunning", mock.AnythingOfType("*ecs.DescribeTasksInput")).Return(nil)

	logger := createTestLogger()

	awsFargate := awsFargate{ecsSvc: mockEcs, logger: logger}

	err := awsFargate.WaitUntilTaskRunning("param1", "param2")

	mockEcs.AssertExpectations(t)
	mockEcs.AssertNumberOfCalls(t, "WaitUntilTasksRunning", 1)

	assert.Nil(t, err, "Should have returned no errors")
}

func TestWaitUntilTaskRunningWithErrorDuringAwsCall(t *testing.T) {

	mockEcs := new(MockECSClient)
	mockEcs.On("WaitUntilTasksRunning", mock.AnythingOfType("*ecs.DescribeTasksInput")).Return(
		fmt.Errorf("Simulated error"),
	)

	logger := createTestLogger()

	awsFargate := awsFargate{ecsSvc: mockEcs, logger: logger}

	err := awsFargate.WaitUntilTaskRunning("param1", "param2")

	mockEcs.AssertExpectations(t)
	mockEcs.AssertNumberOfCalls(t, "WaitUntilTasksRunning", 1)

	assert.NotNil(t, err, "Should have returned error")
	assert.Contains(t, err.Error(), "Simulated error", "Did not return the expected error")
}

func TestStopTaskWithSuccess(t *testing.T) {

	mockEcs := new(MockECSClient)
	mockEcs.On("StopTask", mock.AnythingOfType("*ecs.StopTaskInput")).Return(
		&ecs.StopTaskOutput{}, nil,
	)

	logger := createTestLogger()

	awsFargate := awsFargate{ecsSvc: mockEcs, logger: logger}

	err := awsFargate.StopTask("param1", "param2")

	assert.Nil(t, err, "Should not have returned any error")
}

func TestStopTaskWithErrorDuringAwsCall(t *testing.T) {

	mockEcs := new(MockECSClient)
	mockEcs.On("StopTask", mock.AnythingOfType("*ecs.StopTaskInput")).Return(
		&ecs.StopTaskOutput{}, fmt.Errorf("Simulated error"),
	)

	logger := createTestLogger()

	awsFargate := awsFargate{ecsSvc: mockEcs, logger: logger}

	err := awsFargate.StopTask("param1", "param2")

	assert.NotNil(t, err, "Should have returned error")
	assert.Contains(t, err.Error(), "Simulated error", "Did not return the expected error")
}

func TestGetContainerIPWithSuccessWhenPrivateIp(t *testing.T) {

	expectedIP := "10.0.0.2"
	expectedNetworkIDKey := "networkInterfaceId"
	expectedNetworkIDValue := "net-id"
	mockEcs := new(MockECSClient)
	mockEcs.On("DescribeTasks", mock.AnythingOfType("*ecs.DescribeTasksInput")).Return(
		&ecs.DescribeTasksOutput{
			Tasks: []*ecs.Task{
				&ecs.Task{
					Containers: []*ecs.Container{
						&ecs.Container{
							NetworkInterfaces: []*ecs.NetworkInterface{
								&ecs.NetworkInterface{PrivateIpv4Address: &expectedIP},
							},
						},
					},
					Attachments: []*ecs.Attachment{
						&ecs.Attachment{
							Details: []*ecs.KeyValuePair{
								&ecs.KeyValuePair{Name: &expectedNetworkIDKey, Value: &expectedNetworkIDValue},
							},
						},
					},
				},
			},
		}, nil,
	)

	logger := createTestLogger()

	awsFargate := awsFargate{ecsSvc: mockEcs, logger: logger}

	ip, err := awsFargate.GetContainerIP("param1", "param2", false)

	mockEcs.AssertExpectations(t)

	assert.Nil(t, err, "Should have returned no errors")
	assert.Equal(t, expectedIP, ip, "The IP is different than expected")
}

func TestGetContainerIPWithSuccessWhenPublicIp(t *testing.T) {

	expectedPublicIP := "10.0.0.2"
	expectedPrivateIP := "10.0.0.3"
	expectedNetworkIDKey := "networkInterfaceId"
	expectedNetworkIDValue := "net-id"

	mockEcs := new(MockECSClient)
	mockEcs.On("DescribeTasks", mock.AnythingOfType("*ecs.DescribeTasksInput")).Return(
		&ecs.DescribeTasksOutput{
			Tasks: []*ecs.Task{
				&ecs.Task{
					Containers: []*ecs.Container{
						&ecs.Container{
							NetworkInterfaces: []*ecs.NetworkInterface{
								&ecs.NetworkInterface{PrivateIpv4Address: &expectedPrivateIP},
							},
						},
					},
					Attachments: []*ecs.Attachment{
						&ecs.Attachment{
							Details: []*ecs.KeyValuePair{
								&ecs.KeyValuePair{Name: &expectedNetworkIDKey, Value: &expectedNetworkIDValue},
							},
						},
					},
				},
			},
		}, nil,
	)

	mockEc2 := new(MockEC2Client)
	mockEc2.On("DescribeNetworkInterfaces", mock.AnythingOfType("*ec2.DescribeNetworkInterfacesInput")).Return(
		&ec2.DescribeNetworkInterfacesOutput{
			NetworkInterfaces: []*ec2.NetworkInterface{
				&ec2.NetworkInterface{
					Association: &ec2.NetworkInterfaceAssociation{
						PublicIp: &expectedPublicIP,
					},
				},
			},
		}, nil,
	)

	logger := createTestLogger()

	awsFargate := awsFargate{ecsSvc: mockEcs, ec2Svc: mockEc2, logger: logger}

	ip, err := awsFargate.GetContainerIP("param1", "param2", true)

	mockEcs.AssertExpectations(t)

	assert.Nil(t, err, "Should have returned no errors")
	assert.Equal(t, expectedPublicIP, ip, "The IP is different than expected")
}

func TestGetContainerIPWithErroWhenTryingToFetchTaskInfo(t *testing.T) {

	mockEcs := new(MockECSClient)
	mockEcs.On("DescribeTasks", mock.AnythingOfType("*ecs.DescribeTasksInput")).Return(
		&ecs.DescribeTasksOutput{}, fmt.Errorf("Simulated error"),
	)

	logger := createTestLogger()

	awsFargate := awsFargate{ecsSvc: mockEcs, logger: logger}

	ip, err := awsFargate.GetContainerIP("param1", "param2", false)

	mockEcs.AssertExpectations(t)

	assert.NotNil(t, err, "Should have returned error")
	assert.Contains(t, err.Error(), "Simulated error", "Did not return the expected error")
	assert.Empty(t, ip, "IP should be empty")
}

func TestGetContainerIPWithErrorWhenTryingToGetPublicIp(t *testing.T) {

	expectedPrivateIP := "10.0.0.3"
	expectedNetworkIDKey := "networkInterfaceId"
	expectedNetworkIDValue := "net-id"

	mockEcs := new(MockECSClient)
	mockEcs.On("DescribeTasks", mock.AnythingOfType("*ecs.DescribeTasksInput")).Return(
		&ecs.DescribeTasksOutput{
			Tasks: []*ecs.Task{
				&ecs.Task{
					Containers: []*ecs.Container{
						&ecs.Container{
							NetworkInterfaces: []*ecs.NetworkInterface{
								&ecs.NetworkInterface{PrivateIpv4Address: &expectedPrivateIP},
							},
						},
					},
					Attachments: []*ecs.Attachment{
						&ecs.Attachment{
							Details: []*ecs.KeyValuePair{
								&ecs.KeyValuePair{Name: &expectedNetworkIDKey, Value: &expectedNetworkIDValue},
							},
						},
					},
				},
			},
		}, nil,
	)

	mockEc2 := new(MockEC2Client)
	mockEc2.On("DescribeNetworkInterfaces", mock.AnythingOfType("*ec2.DescribeNetworkInterfacesInput")).Return(
		&ec2.DescribeNetworkInterfacesOutput{}, fmt.Errorf("Simulated error"),
	)

	logger := createTestLogger()

	awsFargate := awsFargate{ecsSvc: mockEcs, ec2Svc: mockEc2, logger: logger}

	ip, err := awsFargate.GetContainerIP("param1", "param2", true)

	mockEcs.AssertExpectations(t)

	assert.NotNil(t, err, "Should have returned error")
	assert.Contains(t, err.Error(), "Simulated error", "Did not return the expected error")
	assert.Empty(t, ip, "IP should be empty")
}
