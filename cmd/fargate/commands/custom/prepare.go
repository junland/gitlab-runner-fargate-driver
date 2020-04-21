package custom

import (
	"fmt"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/aws"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/cli"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/task"
)

// NewPrepareCommand constructs the command line abstraction for the "prepare" stage
func NewPrepareCommand() cli.Command {
	cmd := new(PrepareCommand)
	cmd.abstractCustomCommand.customCommand = cmd

	cmd.newFargate = func(logger logging.Logger, awsRegion string) aws.Fargate {
		return aws.NewFargate(logger, awsRegion)
	}
	cmd.newMetadataManager = func(logger logging.Logger, directory string) task.MetadataManager {
		return task.NewMetadataManager(logger, directory)
	}

	return cli.Command{
		Handler: cmd,
		Config: cli.Config{
			Name:    "prepare",
			Aliases: []string{"p"},
			Usage:   "Prepare the environment for Custom Executor",
			Description: `
This is the implementation of Prepare stage of the Custom Executor.

This command is starting a new AWS Fargate Task that exposes a
container, which will be next used to execute the scripts.

Details about how this command is used can be found at
https://docs.gitlab.com/runner/executors/custom.html#prepare.`,
		},
	}
}

// PrepareCommand provides data and operations related to the "prepare" stage
type PrepareCommand struct {
	abstractCustomCommand

	cfg    config.Global
	logger logging.Logger

	awsFargate      aws.Fargate
	metadataManager task.MetadataManager

	// Wrapping constructors to make easier mocking in the unit tests
	newFargate         func(logger logging.Logger, awsRegion string) aws.Fargate
	newMetadataManager func(logger logging.Logger, directory string) task.MetadataManager
}

// CustomExecute is the "core" of the implementation for the "prepare" stage
func (c *PrepareCommand) CustomExecute(ctx *cli.Context) error {
	err := c.init(ctx)
	if err != nil {
		return fmt.Errorf("initializing PrepareCommand: %w", err)
	}

	c.logger.Info("Executing the command")

	taskARN, err := c.startNewFargateTask(ctx)
	if err != nil {
		c.killFargateTaskOnError(ctx, taskARN, err, "Error starting Fargate task. Will kill the task for cleanup")
		return fmt.Errorf("starting new Fargate task: %w", err)
	}

	// Persist Task ARN to be used by other commands (run / cleanup)
	taskDetails := task.Data{TaskARN: taskARN}
	err = c.persistDataForLaterStages(taskDetails)
	if err != nil {
		c.killFargateTaskOnError(ctx, taskARN, err, "Error persisting task ARN. Will kill the task for cleanup")
		return fmt.Errorf("persisting task ARN for later stages: %w", err)
	}

	containerIP, err := c.waitFargateTaskReady(ctx, taskARN)
	if err != nil {
		c.killFargateTaskOnError(ctx, taskARN, err, "Error during task initialization. Will kill the task for cleanup")
		return fmt.Errorf("waiting Fargate task to be ready: %w", err)
	}

	// Update metadata with the container IP to be used by the "run" command
	taskDetails.ContainerIP = containerIP
	err = c.persistDataForLaterStages(taskDetails)
	if err != nil {
		c.killFargateTaskOnError(ctx, taskARN, err, "Error persisting container IP. Will kill the task for cleanup")
		return fmt.Errorf("persisting container IP for later stages: %w", err)
	}

	return nil
}

func (c *PrepareCommand) init(ctx *cli.Context) error {
	c.cfg = ctx.Config()
	c.logger = ctx.
		Logger().
		WithField("command", "prepare_exec")

	c.awsFargate = c.newFargate(c.logger, c.cfg.Fargate.Region)
	err := c.awsFargate.Init()
	if err != nil {
		return fmt.Errorf("initializing Fargate adapter: %w", err)
	}

	c.metadataManager = c.newMetadataManager(c.logger, c.cfg.TaskMetadata.Directory)

	return nil
}

func (c *PrepareCommand) startNewFargateTask(ctx *cli.Context) (string, error) {
	c.logger.Info("Starting new Fargate task")

	taskDefinition := c.cfg.Fargate.TaskDefinition
	connection := aws.ConnectionSettings{
		Cluster:        c.cfg.Fargate.Cluster,
		Subnet:         c.cfg.Fargate.Subnet,
		SecurityGroup:  c.cfg.Fargate.SecurityGroup,
		EnablePublicIP: c.cfg.Fargate.EnablePublicIP,
	}

	taskARN, err := c.awsFargate.RunTask(ctx.Ctx, taskDefinition, connection)
	if err != nil {
		return taskARN, fmt.Errorf("running new task on Fargate: %w", err)
	}

	return taskARN, nil
}

func (c *PrepareCommand) killFargateTaskOnError(ctx *cli.Context, taskARN string, err error, logMessage string) {
	if taskARN != "" {
		c.logger.
			WithField("taskARN", taskARN).
			WithError(err).
			Error(logMessage)

		stopErr := c.awsFargate.StopTask(ctx.Ctx, taskARN, c.cfg.Fargate.Cluster)
		if stopErr != nil {
			c.logger.
				WithField("taskARN", taskARN).
				WithError(stopErr).
				Error("Error during stop task")
		}
	}
}

func (c *PrepareCommand) persistDataForLaterStages(taskDetails task.Data) error {
	c.logger.
		WithField("taskARN", taskDetails.TaskARN).
		Info("Persisting data that will be used by other commands")

	err := c.metadataManager.Persist(taskDetails)
	if err != nil {
		return fmt.Errorf("persisting metadata: %w", err)
	}

	return nil
}

func (c *PrepareCommand) waitFargateTaskReady(ctx *cli.Context, taskARN string) (string, error) {
	c.logger.Info("Waiting Fargate task to be ready")

	var containerIP string

	// Wait the task to be in "running" state
	err := c.awsFargate.WaitUntilTaskRunning(ctx.Ctx, taskARN, c.cfg.Fargate.Cluster)
	if err != nil {
		return containerIP, fmt.Errorf("waiting for Fargate task to be in running state: %w", err)
	}

	// Get the container IP
	containerIP, err = c.awsFargate.GetContainerIP(
		ctx.Ctx,
		taskARN,
		c.cfg.Fargate.Cluster,
		c.cfg.Fargate.EnablePublicIP,
	)
	if err != nil {
		return containerIP, fmt.Errorf("fetching the container IP: %w", err)
	}

	return containerIP, nil
}
