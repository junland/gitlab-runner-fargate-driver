// Package aws provides abstractions that will be used for managing Amazon AWS resources
package aws

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/sts"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
)

var (
	defaultContainerName = "ci-coordinator"

	// ErrNotInitialized is returned when the fargate methods are invoked without initialization
	ErrNotInitialized = errors.New("fargate adapter is not initialized")
	ErrTaskRunFailure = errors.New("fargate task run failure")
	ErrNoTaskRun      = errors.New("fargate no tasks could be run")
)

// Fargate should be used to manage AWS Fargate Tasks (start, stop, etc)
type Fargate interface {
	// RunTask starts a new task in a pre configured Fargate
	RunTask(ctx context.Context, taskSettings TaskSettings, connection ConnectionSettings) (string, error)

	// WaitUntilTaskRunning blocks the request until the task is in "running"
	// state or failed to reach this state
	WaitUntilTaskRunning(ctx context.Context, taskARN string, cluster string, taskWaitConfig config.TaskWaitConfig) error

	// WaitUntilTaskHealthy blocks until the task is in "healthy" state if a
	// healthcheck is defined. It returns an error if the task failed to reach
	// a healthy state in the allotted time.
	WaitUntilTaskHealthy(ctx context.Context, taskARN string, cluster string, taskHealthWaitConfig config.TaskHealthWaitConfig) error

	// RunTask stops a specified Fargate task
	StopTask(ctx context.Context, taskARN, cluster, stopReason string) error

	// GetContainerIP returns the IP of the container related to the specified task
	GetContainerIP(ctx context.Context, taskARN string, cluster string, enablePublicIP bool, usePublicIP bool) (string, error)

	// Init initialize variables and executes necessary procedures
	Init() error
}

// TaskSettings centralizes attributes related to the task configuration
type TaskSettings struct {
	Cluster              string
	TaskDefinition       string
	PlatformVersion      string
	EnvironmentVariables map[string]string
	// Indicates whether to propagate the tags from the task definition to the running task
	PropagateTags bool
}

// ConnectionSettings centralizes attributes related to the task's network configuration
type ConnectionSettings struct {
	Subnets        []*string
	SecurityGroups []*string
	EnablePublicIP bool
	UsePublicIP    bool
}

type ecsClient interface {
	DescribeTasksRequest(input *ecs.DescribeTasksInput) (req *request.Request, output *ecs.DescribeTasksOutput)
	WaitUntilTaskHealthyWithContext(ctx aws.Context, input *ecs.DescribeTasksInput, opts ...request.WaiterOption) error
	RunTaskWithContext(aws.Context, *ecs.RunTaskInput, ...request.Option) (*ecs.RunTaskOutput, error)
	WaitUntilTasksRunningWithContext(aws.Context, *ecs.DescribeTasksInput, ...request.WaiterOption) error
	StopTaskWithContext(aws.Context, *ecs.StopTaskInput, ...request.Option) (*ecs.StopTaskOutput, error)
	DescribeTasksWithContext(aws.Context, *ecs.DescribeTasksInput, ...request.Option) (*ecs.DescribeTasksOutput, error)
	DescribeTaskDefinitionWithContext(ctx aws.Context, input *ecs.DescribeTaskDefinitionInput, opts ...request.Option) (*ecs.DescribeTaskDefinitionOutput, error)
}

type ec2Client interface {
	DescribeNetworkInterfacesWithContext(aws.Context, *ec2.DescribeNetworkInterfacesInput, ...request.Option) (*ec2.DescribeNetworkInterfacesOutput, error)
}

// We do this for the time being because
// `github.com/aws/aws-sdk-go/service/ecs.ECS` does not have a
// `WaitUntilTaskHealthyWithContext` API, so we have to implement it here.
type extendedEcsSvc struct {
	*ecs.ECS
}

type awsFargate struct {
	logger         logging.Logger
	awsRegion      string
	roleARN        string
	stsSessionVars string
	ecsSvc         ecsClient
	ec2Svc         ec2Client

	// The AWS NewSession function was encapsulated into the sessionCreator
	// to make easier creating unit tests
	sessionCreator func(awsRegion string) (*session.Session, error)
}

// NewFargate is a constructor for the concrete type of the Fargate interface
func NewFargate(logger logging.Logger, awsRegion, roleARN, stsSessionVars string) Fargate {
	awsFargate := new(awsFargate)

	awsFargate.logger = logger
	awsFargate.awsRegion = awsRegion
	awsFargate.sessionCreator = func(awsRegion string) (*session.Session, error) {
		return session.NewSession(
			&aws.Config{
				Region: aws.String(awsRegion),
			},
		)
	}
	awsFargate.roleARN = roleARN
	awsFargate.stsSessionVars = stsSessionVars

	return awsFargate
}

func (a *awsFargate) Init() error {
	sess, err := a.sessionCreator(a.awsRegion)
	if err != nil {
		return fmt.Errorf("couldn't create AWS session: %w", err)
	}

	a.ecsSvc = extendedEcsSvc{ecs.New(sess)}
	a.ec2Svc = ec2.New(sess)

	if a.roleARN != "" {
		if a.stsSessionVars != "" && a.roleARN == "" {
			return fmt.Errorf("stsSessionVars is only intended to be used with roleARN. Please provide a value for roleARN")
		}

		a.logger.WithField("role-arn", a.roleARN).Info(`[Init] Assuming role`)

		creds := *stscreds.NewCredentials(sess, a.roleARN)

		if a.stsSessionVars != "" {
			a.logger.WithField("variables", a.stsSessionVars).Info(`[Init] Adding tags to session`)

			tags, tagKeys := getSTSTags(a.stsSessionVars)
			creds = *stscreds.NewCredentials(sess, a.roleARN, func(p *stscreds.AssumeRoleProvider) {
				p.Tags = tags
				p.TransitiveTagKeys = tagKeys
			})
		}

		a.ecsSvc = extendedEcsSvc{ecs.New(sess, &aws.Config{Credentials: &creds})}
		a.ec2Svc = ec2.New(sess, &aws.Config{Credentials: &creds})
	}

	return nil
}

func getSTSTags(stsSessionVars string) ([]*sts.Tag, []*string) {
	stsTags, tagKeys := []*sts.Tag{}, []*string{}
	if stsSessionVars != "" {
		for _, v := range strings.Split(stsSessionVars, ",") {
			stsTags = append(stsTags, &sts.Tag{Key: aws.String(v), Value: aws.String(os.Getenv("CUSTOM_ENV_" + v))})
			tagKeys = append(tagKeys, aws.String(v))
		}
	}
	return stsTags, tagKeys
}

//nolint:gocyclo
func (a *awsFargate) RunTask(ctx context.Context, taskSettings TaskSettings, connection ConnectionSettings) (string, error) {
	err := a.errIfNotInitialized()
	if err != nil {
		return "", fmt.Errorf("could not start AWS Fargate Task: %w", err)
	}

	a.logger.Debug("[RunTask] Will start a new Fargate Task")

	publicIP := "DISABLED"
	if connection.EnablePublicIP {
		publicIP = "ENABLED"
	}

	var platformVersion *string
	if taskSettings.PlatformVersion != "" {
		platformVersion = &taskSettings.PlatformVersion
	}

	taskInput := ecs.RunTaskInput{
		TaskDefinition: &taskSettings.TaskDefinition,
		Cluster:        &taskSettings.Cluster,
		NetworkConfiguration: &ecs.NetworkConfiguration{
			AwsvpcConfiguration: &ecs.AwsVpcConfiguration{
				SecurityGroups: connection.SecurityGroups,
				Subnets:        connection.Subnets,
				AssignPublicIp: &publicIP,
			},
		},
		Overrides:       a.processEnvVariablesToInject(taskSettings.EnvironmentVariables),
		PlatformVersion: platformVersion,
	}

	if taskSettings.PropagateTags {
		taskInput.SetPropagateTags(ecs.PropagateTagsTaskDefinition)
	}

	taskOutput, err := a.ecsSvc.RunTaskWithContext(ctx, &taskInput)
	if err != nil {
		return "", fmt.Errorf("error starting AWS Fargate Task: %w", err)
	}

	if len(taskOutput.Failures) > 0 {
		a.logFailures(taskOutput.Failures)
		return "", ErrTaskRunFailure
	}

	if len(taskOutput.Tasks) == 0 {
		return "", ErrNoTaskRun
	}

	taskARN := *taskOutput.Tasks[0].TaskArn

	a.logger.
		WithField("task-arn", taskARN).
		Debug("[RunTask] Fargate Task started with success")

	return taskARN, nil
}

func (a *awsFargate) logFailures(failures []*ecs.Failure) {
	for _, failure := range failures {
		a.logger.
			WithField("failure", failure).
			Error("[RunTask] Fargate Task run failure")
	}
}

func (a *awsFargate) errIfNotInitialized() error {
	if a.ecsSvc != nil && a.ec2Svc != nil {
		return nil
	}

	return ErrNotInitialized
}

func (a *awsFargate) processEnvVariablesToInject(envVars map[string]string) *ecs.TaskOverride {
	if (envVars == nil) || (len(envVars) == 0) {
		return nil
	}

	environmentVars := make([]*ecs.KeyValuePair, 0)
	for key, value := range envVars {
		name := key
		v := value
		environmentVars = append(environmentVars, &ecs.KeyValuePair{
			Name:  &name,
			Value: &v,
		})
	}

	taskOverride := &ecs.TaskOverride{
		ContainerOverrides: []*ecs.ContainerOverride{
			{
				Name:        &defaultContainerName,
				Environment: environmentVars,
			},
		},
	}

	a.logger.Debug("[processEnvVariablesToInject] Environment variables processed with success")

	return taskOverride
}

func (a *awsFargate) WaitUntilTaskRunning(ctx context.Context, taskARN string, cluster string, taskWaitConfig config.TaskWaitConfig) error {
	err := a.errIfNotInitialized()
	if err != nil {
		return fmt.Errorf("could not wait AWS Fargate task: %w", err)
	}

	a.logger.
		WithField("task-arn", taskARN).
		Debug(`[WaitUntilTaskRunning] Will wait until Fargate task is in "Running" state`)

	input := ecs.DescribeTasksInput{
		Cluster: &cluster,
		Tasks:   []*string{&taskARN},
	}

	waiter := func(w *request.Waiter) {
		w.MaxAttempts = taskWaitConfig.MaxAttempts
		w.Delay = request.ConstantWaiterDelay(time.Duration(taskWaitConfig.WaitTimeout) * time.Second)
	}
	err = a.ecsSvc.WaitUntilTasksRunningWithContext(ctx, &input, waiter)
	if err != nil {
		return fmt.Errorf(`error waiting AWS Fargate Task %q to be in "Running" state: %w`, taskARN, err)
	}

	a.logger.
		WithField("task-arn", taskARN).
		Debug(`[WaitUntilTaskRunning] Fargate Task in "Running" state`)

	return nil
}

func (a *awsFargate) WaitUntilTaskHealthy(ctx context.Context, taskARN string, cluster string, taskHealthWaitConfig config.TaskHealthWaitConfig) error {
	taskDetails, err := a.getTaskDetails(ctx, taskARN, cluster)
	if err != nil {
		return fmt.Errorf("error accessing task details %q: %w", taskARN, err)
	}

	taskDefinitionARN := *taskDetails.Tasks[0].TaskDefinitionArn
	taskDefinitionDetails, err := a.getTaskDefinitionDetails(ctx, taskDefinitionARN)
	if err != nil {
		return fmt.Errorf("error accessing task definition details %q: %w", taskDefinitionARN, err)
	}

	hasHealthCheck, err := a.IsTaskDefinitionWithHealthCheckDefined(taskDefinitionDetails)
	if err != nil {
		return fmt.Errorf("error extracting health check: %w", err)
	}

	if !hasHealthCheck {
		a.logger.
			WithField("task-arn", taskARN).
			Info(`[WaitUntilTaskHealthy] No healthcheck container definition specified; skipping waiting for "Healthy" status`)
		return nil
	}

	ctx, cancel := context.WithDeadline(
		ctx,
		time.Now().Add(taskHealthWaitConfig.CheckInterval*time.Duration(taskHealthWaitConfig.Retries)))
	defer cancel()

	err = a.ecsSvc.WaitUntilTaskHealthyWithContext(
		ctx,
		&ecs.DescribeTasksInput{
			Cluster: &cluster,
			Tasks:   []*string{&taskARN},
		},
		func(w *request.Waiter) {
			w.MaxAttempts = taskHealthWaitConfig.Retries
			w.Delay = request.ConstantWaiterDelay(taskHealthWaitConfig.CheckInterval)
		},
	)

	if err != nil {
		return fmt.Errorf(`error waiting AWS Fargate Task health %q to be in "Healthy" state: %w`, taskARN, err)
	}

	return nil
}

func (a *awsFargate) StopTask(ctx context.Context, taskARN, cluster, stopReason string) error {
	err := a.errIfNotInitialized()
	if err != nil {
		return fmt.Errorf("could not stop AWS Fargate Task: %w", err)
	}

	a.logger.
		WithField("task-arn", taskARN).
		Debug("[StopTask] Will stop the task")

	input := ecs.StopTaskInput{
		Cluster: &cluster,
		Task:    &taskARN,
		Reason:  &stopReason,
	}

	_, err = a.ecsSvc.StopTaskWithContext(ctx, &input)
	if err != nil {
		return fmt.Errorf("error stopping AWS Fargate Task %q: %w", taskARN, err)
	}

	a.logger.
		WithField("task-arn", taskARN).
		Debug("[StopTask] Fargate Task stopped with success")

	return nil
}

func (a *awsFargate) GetContainerIP(ctx context.Context, taskARN string, cluster string, _ bool, usePublicIP bool) (string, error) {
	err := a.errIfNotInitialized()
	if err != nil {
		return "", fmt.Errorf("could not get container IP: %w", err)
	}

	a.logger.
		WithField("task-arn", taskARN).
		Debug("[GetContainerIP] Will get the IP for the task")

	taskDetails, err := a.getTaskDetails(ctx, taskARN, cluster)
	if err != nil {
		return "", fmt.Errorf("error accessing information about the task %q: %w", taskARN, err)
	}

	if !usePublicIP {
		privateIP := a.extractPrivateIP(taskDetails)
		return privateIP, nil
	}

	networkInterfaceID := a.extractNetworkInterfaceID(taskDetails)
	publicIP, err := a.getPublicIP(ctx, networkInterfaceID)
	if err != nil {
		return "", fmt.Errorf("error trying to get the public IP: %w", err)
	}

	a.logger.Debug("[GetContainerIP] Ip fetched with success")

	return publicIP, nil
}

func (a *awsFargate) getTaskDefinitionDetails(ctx context.Context, taskDefinitionARN string) (*ecs.DescribeTaskDefinitionOutput, error) {
	dt, err := a.ecsSvc.DescribeTaskDefinitionWithContext(
		ctx,
		&ecs.DescribeTaskDefinitionInput{
			TaskDefinition: &taskDefinitionARN,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to describe task definition: %w", err)
	}

	a.logger.Debug("[getTaskDefinitionDetails] Finished fetching the task definition details")

	return dt, nil
}

func (a *awsFargate) getTaskDetails(ctx context.Context, taskARN string, cluster string) (*ecs.DescribeTasksOutput, error) {
	dt, err := a.ecsSvc.DescribeTasksWithContext(
		ctx,
		&ecs.DescribeTasksInput{
			Cluster: &cluster,
			Tasks:   []*string{&taskARN},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error during call to describe fargate task: %w", err)
	}

	if len(dt.Tasks) == 0 {
		return nil, fmt.Errorf("no tasks returned for task arn %q", taskARN)
	}

	return dt, nil
}

func (a *awsFargate) IsTaskDefinitionWithHealthCheckDefined(descOut *ecs.DescribeTaskDefinitionOutput) (bool, error) {
	if len(descOut.TaskDefinition.ContainerDefinitions) == 0 {
		return false, fmt.Errorf("no container definition found in task %q", *descOut.TaskDefinition.TaskDefinitionArn)
	}

	for _, e := range descOut.TaskDefinition.ContainerDefinitions {
		if e.HealthCheck != nil {
			return true, nil
		}
	}
	return false, nil
}

func (a *awsFargate) extractPrivateIP(descOut *ecs.DescribeTasksOutput) string {
	privateIP := *descOut.
		Tasks[0].
		Containers[0].
		NetworkInterfaces[0].
		PrivateIpv4Address

	a.logger.
		WithField("private-ip", privateIP).
		Debug("[extractPrivateIP] Finished extracting the private IP")

	return privateIP
}

func (a *awsFargate) extractNetworkInterfaceID(descOut *ecs.DescribeTasksOutput) string {
	var networkInterfaceID string

	for _, detail := range descOut.Tasks[0].Attachments[0].Details {
		if *detail.Name == "networkInterfaceId" {
			networkInterfaceID = *detail.Value
			break
		}
	}

	a.logger.
		WithField("network-interface-id", networkInterfaceID).
		Debug("[extractNetworkInterfaceID] Finished extracting the network interface ID")

	return networkInterfaceID
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

func (client extendedEcsSvc) WaitUntilTaskHealthyWithContext(ctx aws.Context, input *ecs.DescribeTasksInput, opts ...request.WaiterOption) error {
	w := request.Waiter{
		Name: "WaitUntilTaskHealthyWithContext",
		Acceptors: []request.WaiterAcceptor{
			{
				State:   request.FailureWaiterState,
				Matcher: request.PathAnyWaiterMatch, Argument: "tasks[].healthStatus",
				Expected: "UNHEALTHY",
			},
			{
				State:   request.FailureWaiterState,
				Matcher: request.PathAnyWaiterMatch, Argument: "failures[].reason",
				Expected: "UNKNOWN",
			},
			{
				State:   request.SuccessWaiterState,
				Matcher: request.PathAllWaiterMatch, Argument: "tasks[].healthStatus",
				Expected: "HEALTHY",
			},
		},
		Logger: aws.NewDefaultLogger(),
		NewRequest: func(opts []request.Option) (*request.Request, error) {
			var inCpy *ecs.DescribeTasksInput
			if input != nil {
				tmp := *input
				inCpy = &tmp
			}
			req, _ := client.DescribeTasksRequest(inCpy)
			req.SetContext(ctx)
			req.ApplyOptions(opts...)
			return req, nil
		},
	}

	w.ApplyOptions(opts...)

	return w.WaitWithContext(ctx)
}
