// Package aws provides abstractions that will be used for managing Amazon AWS resources
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
)

// Fargate should be used to manage AWS Fargate Tasks (start, stop, etc)
type Fargate interface {
	// RunTask starts a new task in a pre configured Fargate
	RunTask(ctx context.Context, taskDefinition string, cluster string, subnet string,
		securityGroup string, enablePublicIP bool) (string, error)

	// WaitUntilTaskRunning blocks the request until the task is in "running"
	// state or failed to reach this state
	WaitUntilTaskRunning(ctx context.Context, taskARN string, cluster string) error

	// RunTask stops a specified Fargate task
	StopTask(ctx context.Context, taskARN string, cluster string) error

	// GetContainerIP returns the IP of the container related to the specified task
	GetContainerIP(ctx context.Context, taskARN string, cluster string, usePublicIP bool) (string, error)

	// Init initialize variables and executes necessary procedures
	Init(logger logging.Logger, awsRegion string) error
}

// ECSClient is an adapter for the ECS client interface
type ECSClient interface {
	RunTaskWithContext(aws.Context, *ecs.RunTaskInput, ...request.Option) (*ecs.RunTaskOutput, error)
	WaitUntilTasksRunningWithContext(aws.Context, *ecs.DescribeTasksInput, ...request.WaiterOption) error
	StopTaskWithContext(aws.Context, *ecs.StopTaskInput, ...request.Option) (*ecs.StopTaskOutput, error)
	DescribeTasksWithContext(aws.Context, *ecs.DescribeTasksInput, ...request.Option) (*ecs.DescribeTasksOutput, error)
}

// EC2Client is an adapter for the EC2 client interface
type EC2Client interface {
	DescribeNetworkInterfacesWithContext(aws.Context, *ec2.DescribeNetworkInterfacesInput, ...request.Option) (*ec2.DescribeNetworkInterfacesOutput, error)
}

type awsFargate struct {
	logger logging.Logger
	ecsSvc ECSClient
	ec2Svc EC2Client

	// The AWS NewSession function was encapsulated into the sessionCreator
	// to make easier creating unit tests
	sessionCreator func(awsRegion string) (*session.Session, error)
}

// NewFargate is a constructor for the concrete type of the Fargate interface
func NewFargate() Fargate {
	awsFargate := awsFargate{}

	awsFargate.sessionCreator = func(awsRegion string) (*session.Session, error) {
		return session.NewSession(
			&aws.Config{
				Region: aws.String(awsRegion),
			},
		)
	}

	return &awsFargate
}

func (a *awsFargate) Init(logger logging.Logger, awsRegion string) error {
	sess, err := a.sessionCreator(awsRegion)
	if err != nil {
		return fmt.Errorf("couldn't create AWS session: %w", err)
	}

	a.logger = logger
	a.ecsSvc = ecs.New(sess)
	a.ec2Svc = ec2.New(sess)

	return nil
}

func (a *awsFargate) RunTask(ctx context.Context, taskDefinition string, cluster string, subnet string,
	securityGroup string, enablePublicIP bool) (string, error) {
	a.logger.Debug("[RunTask] Will start a new Fargate Task")

	publicIP := "DISABLED"
	if enablePublicIP {
		publicIP = "ENABLED"
	}
	taskInput := ecs.RunTaskInput{
		TaskDefinition: &taskDefinition,
		Cluster:        &cluster,
		NetworkConfiguration: &ecs.NetworkConfiguration{
			AwsvpcConfiguration: &ecs.AwsVpcConfiguration{
				SecurityGroups: []*string{&securityGroup},
				Subnets:        []*string{&subnet},
				AssignPublicIp: &publicIP,
			},
		},
	}

	taskOutput, err := a.ecsSvc.RunTaskWithContext(ctx, &taskInput)
	if err != nil {
		return "", fmt.Errorf("error starting AWS Fargate Task: %w", err)
	}

	taskARN := *taskOutput.Tasks[0].TaskArn

	a.logger.
		WithField("task-arn", taskARN).
		Debug("[RunTask] Fargate Task started with success")

	return taskARN, nil
}

func (a *awsFargate) WaitUntilTaskRunning(ctx context.Context, taskARN string, cluster string) error {
	a.logger.
		WithField("task-arn", taskARN).
		Debug(`[WaitUntilTaskRunning] Will wait until Fargate task is in "Running" state`)

	input := ecs.DescribeTasksInput{
		Cluster: &cluster,
		Tasks:   []*string{&taskARN},
	}

	err := a.ecsSvc.WaitUntilTasksRunningWithContext(ctx, &input)
	if err != nil {
		return fmt.Errorf(`error waiting AWS Fargate Task %q to be in "Running" state: %w`, taskARN, err)
	}

	a.logger.
		WithField("task-arn", taskARN).
		Debug(`[WaitUntilTaskRunning] Fargate Task in "Running" state`)

	return nil
}

func (a *awsFargate) StopTask(ctx context.Context, taskARN string, cluster string) error {
	a.logger.
		WithField("task-arn", taskARN).
		Debug("[StopTask] Will stop the task")

	input := ecs.StopTaskInput{
		Cluster: &cluster,
		Task:    &taskARN,
	}

	_, err := a.ecsSvc.StopTaskWithContext(ctx, &input)
	if err != nil {
		return fmt.Errorf("error stopping AWS Fargate Task %q: %w", taskARN, err)
	}

	a.logger.
		WithField("task-arn", taskARN).
		Debug("[StopTask] Fargate Task stopped with success")

	return nil
}

func (a *awsFargate) GetContainerIP(ctx context.Context, taskARN string, cluster string, usePublicIP bool) (string, error) {
	a.logger.
		WithField("task-arn", taskARN).
		Debug("[GetContainerIP] Will get the IP for the task")

	privateIP, networkInterfaceID, err := a.getPrivateIPAndNetworkInterface(ctx, taskARN, cluster)
	if err != nil {
		return "", fmt.Errorf("error accessing information for the task %q: %w", taskARN, err)
	}

	if !usePublicIP {
		return privateIP, nil
	}

	publicIP, err := a.getPublicIP(ctx, networkInterfaceID)
	if err != nil {
		return "", fmt.Errorf("error trying to get the public IP: %w", err)
	}

	a.logger.Debug("[GetContainerIP] Ip fetched with success")

	return publicIP, nil
}

func (a *awsFargate) getPublicIP(ctx context.Context, networkInterfaceID string) (string, error) {
	dni, err := a.ec2Svc.DescribeNetworkInterfacesWithContext(
		ctx,
		&ec2.DescribeNetworkInterfacesInput{
			NetworkInterfaceIds: []*string{&networkInterfaceID},
		},
	)
	if err != nil {
		return "", fmt.Errorf("error reading network interfaces: %w", err)
	}

	publicIP := *dni.NetworkInterfaces[0].Association.PublicIp
	a.logger.
		WithField("public-ip", publicIP).
		Debug("[getPublicIP] Finished fetching the public IP")

	return publicIP, nil
}

func (a *awsFargate) getPrivateIPAndNetworkInterface(ctx context.Context, taskARN string, cluster string) (string, string, error) {
	dt, err := a.ecsSvc.DescribeTasksWithContext(
		ctx,
		&ecs.DescribeTasksInput{
			Cluster: &cluster,
			Tasks:   []*string{&taskARN},
		},
	)
	if err != nil {
		return "", "", fmt.Errorf("error fetching information about the Fargate task: %w", err)
	}

	privateIP := *dt.Tasks[0].Containers[0].NetworkInterfaces[0].PrivateIpv4Address
	a.logger.
		WithField("private-ip", privateIP).
		Debug("[getPrivateIPAndNetworkInterface] Finished fetching the private IP")

	networkInterfaceID := a.getNetworkInterfaceID(dt)
	a.logger.
		WithField("network-interface-id", networkInterfaceID).
		Debug("[getPrivateIPAndNetworkInterface] Finished fetching the network interface ID")

	return privateIP, networkInterfaceID, nil
}

func (a *awsFargate) getNetworkInterfaceID(descOut *ecs.DescribeTasksOutput) string {
	var networkInterfaceID string

	for _, detail := range descOut.Tasks[0].Attachments[0].Details {
		if *detail.Name == "networkInterfaceId" {
			networkInterfaceID = *detail.Value
			break
		}
	}

	return networkInterfaceID
}
