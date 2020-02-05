// Package aws provides abstractions that will be used for managing Amazon AWS resources
package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ecs/ecsiface"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
)

// Fargate should be used to manage AWS Fargate Tasks (start, stop, etc)
type Fargate interface {

	// RunTask starts a new task in a pre configured Fargate
	RunTask(taskDefinition string, cluster string, subnet string,
		securityGroup string, enablePublicIP bool) (taskArn string, err error)

	// WaitUntilTaskRunning blocks the request until the task is in "running"
	// state or failed to reach this state
	WaitUntilTaskRunning(taskArn string, clusterName string) error

	// RunTask stops a specified Fargate task
	StopTask(taskArn string, clusterName string) error

	// GetContainerIP returns the IP of the container related to the specified task
	GetContainerIP(taskArn string, clusterName string, usePublicIP bool) (string, error)

	// Init initialize variables and executes necessary procedures
	Init(logger logging.Logger, awsRegion string) error
}

type awsFargate struct {
	logger logging.Logger
	ecsSvc ecsiface.ECSAPI
	ec2Svc ec2iface.EC2API
}

var (
	// The NewSession function was encapsulated to make easier creating unit tests
	sessionCreator = func(awsRegion string) (*session.Session, error) {
		return session.NewSession(
			&aws.Config{
				Region: aws.String(awsRegion),
			},
		)
	}
)

// NewFargate is a constructor for the concrete type of the Fargate interface
func NewFargate() Fargate {
	return &awsFargate{}
}

func (a *awsFargate) Init(logger logging.Logger, awsRegion string) error {

	sess, err := sessionCreator(awsRegion)
	if err != nil {
		return err
	}

	a.logger = logger
	a.ecsSvc = ecs.New(sess)
	a.ec2Svc = ec2.New(sess)

	return nil
}

func (a *awsFargate) RunTask(taskDefinition string, cluster string, subnet string,
	securityGroup string, enablePublicIP bool) (taskArn string, err error) {

	a.logger.Debug("[RunTask] Will start a new Fargate Task")

	var publicIP string
	if enablePublicIP {
		publicIP = "ENABLED"
	} else {
		publicIP = "DISABLED"
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

	taskOutput, err := a.ecsSvc.RunTask(&taskInput)
	if err != nil {
		return "", fmt.Errorf("Error starting AWS Fargate Task: %w", err)
	}

	taskArn = *taskOutput.Tasks[0].TaskArn

	a.logger.Debugf("[RunTask] Fargate Task %s started with success", taskArn)

	return taskArn, nil
}

func (a *awsFargate) WaitUntilTaskRunning(taskArn string, clusterName string) error {

	a.logger.Debugf(
		"[WaitUntilTaskRunning] Will wait until Fargate task %s is in \"Running\" state",
		taskArn,
	)

	input := ecs.DescribeTasksInput{
		Cluster: &clusterName,
		Tasks:   []*string{&taskArn},
	}

	err := a.ecsSvc.WaitUntilTasksRunning(&input)
	if err != nil {
		return fmt.Errorf("Error waiting AWS Fargate Task %s to be in \"Running\" state: %w", taskArn, err)
	}

	a.logger.Debugf("[WaitUntilTaskRunning] Fargate Task in \"Running\" state %s", taskArn)

	return nil
}

func (a *awsFargate) StopTask(taskArn string, clusterName string) error {

	a.logger.Debug("[StopTask] Will stop the task %s", taskArn)

	input := ecs.StopTaskInput{
		Cluster: &clusterName,
		Task:    &taskArn,
	}

	_, err := a.ecsSvc.StopTask(&input)
	if err != nil {
		return fmt.Errorf("Error stopping AWS Fargate Task %s: %w", taskArn, err)
	}

	a.logger.Debugf("[StopTask] Fargate Task %s stopped with success", taskArn)

	return nil
}

func (a *awsFargate) GetContainerIP(taskArn string, clusterName string, usePublicIP bool) (string, error) {

	a.logger.Debugf("[GetContainerIP] Will get the IP for the task %s", taskArn)

	privateIP, networkInterfaceID, err := a.getPrivateIPAndNetworkInterface(taskArn, clusterName)
	if err != nil {
		return "", fmt.Errorf("Error accessing information for the task %s: %w", taskArn, err)
	}

	if !usePublicIP {
		return privateIP, nil
	}

	publicIP, err := a.getPublicIP(networkInterfaceID)
	if err != nil {
		return "", fmt.Errorf("Error trying to get the public IP: %w", err)
	}

	a.logger.Debug("[GetContainerIP] Ip fetched with success")

	return publicIP, nil
}

func (a *awsFargate) getPublicIP(networkInterfaceID string) (string, error) {

	dni, err := a.ec2Svc.DescribeNetworkInterfaces(
		&ec2.DescribeNetworkInterfacesInput{
			NetworkInterfaceIds: []*string{&networkInterfaceID},
		},
	)
	if err != nil {
		return "", err
	}

	publicIP := *dni.NetworkInterfaces[0].Association.PublicIp
	a.logger.Debugf("[getPublicIP] Public IP: %s", publicIP)

	return publicIP, nil
}

func (a *awsFargate) getPrivateIPAndNetworkInterface(taskArn string, clusterName string) (string, string, error) {

	dt, err := a.ecsSvc.DescribeTasks(
		&ecs.DescribeTasksInput{
			Cluster: &clusterName,
			Tasks:   []*string{&taskArn},
		},
	)
	if err != nil {
		return "", "", err
	}

	privateIP := *dt.Tasks[0].Containers[0].NetworkInterfaces[0].PrivateIpv4Address
	a.logger.Debugf("[getPrivateIPAndNetworkInterface] Private IP: %s", privateIP)

	networkInterfaceID := a.getNetworkInterfaceID(dt)
	a.logger.Debugf("[getPrivateIPAndNetworkInterface] Network interface ID: %s", networkInterfaceID)

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
