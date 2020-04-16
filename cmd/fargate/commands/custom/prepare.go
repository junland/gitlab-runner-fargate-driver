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

	taskDetails, err := c.prepareNewFargateTask(ctx)
	if err != nil {
		if taskDetails.TaskARN != "" {
			c.logger.
				WithField("taskARN", taskDetails.TaskARN).
				WithError(err).
				Error("Error during task initialization. Will kill the task for cleanup")
			c.stopTask(ctx, taskDetails.TaskARN, c.cfg.FargateConfig.Cluster)
		}

		return fmt.Errorf("preparing new Fargate task: %w", err)
	}

	err = c.persistDataForLaterStages(taskDetails)
	if err != nil {
		c.logger.
			WithField("taskARN", taskDetails.TaskARN).
			WithError(err).
			Error("Error persisting task data necessary for next stages. Will kill the task for cleanup")
		c.stopTask(ctx, taskDetails.TaskARN, c.cfg.FargateConfig.Cluster)

		return fmt.Errorf("persisting task data for later stages: %w", err)
	}

	return nil
}

func (c *PrepareCommand) init(ctx *cli.Context) error {
	c.cfg = ctx.Config()
	c.logger = ctx.
		Logger().
		WithField("command", "prepare_exec")

	c.awsFargate = c.newFargate(c.logger, c.cfg.FargateConfig.Region)
	err := c.awsFargate.Init()
	if err != nil {
		return fmt.Errorf("initializing Fargate adapter: %w", err)
	}

	c.metadataManager = c.newMetadataManager(c.logger, c.cfg.TaskMetadata.Directory)

	return nil
}

func (c *PrepareCommand) prepareNewFargateTask(ctx *cli.Context) (task.Data, error) {
	c.logger.Info("Preparing Fargate task")

	taskDefinition := c.cfg.FargateConfig.TaskDefinition
	connection := aws.ConnectionSettings{
		Cluster:        c.cfg.FargateConfig.Cluster,
		Subnet:         c.cfg.FargateConfig.Subnet,
		SecurityGroup:  c.cfg.FargateConfig.SecurityGroup,
		EnablePublicIP: c.cfg.FargateConfig.EnablePublicIP,
	}

	var err error
	var taskDetails task.Data

	// Start the Fargate task
	taskDetails.TaskARN, err = c.awsFargate.RunTask(ctx.Ctx, taskDefinition, connection)
	if err != nil {
		return taskDetails, fmt.Errorf("starting Fargate task: %w", err)
	}

	// Wait the task to be in "running" state
	err = c.awsFargate.WaitUntilTaskRunning(ctx.Ctx, taskDetails.TaskARN, connection.Cluster)
	if err != nil {
		return taskDetails, fmt.Errorf("waiting for Fargate task to be in running state: %w", err)
	}

	// Get the container IP
	taskDetails.ContainerIP, err = c.awsFargate.GetContainerIP(
		ctx.Ctx,
		taskDetails.TaskARN,
		connection.Cluster,
		connection.EnablePublicIP,
	)
	if err != nil {
		return taskDetails, fmt.Errorf("fetching the container IP: %w", err)
	}

	return taskDetails, nil
}

func (c *PrepareCommand) stopTask(ctx *cli.Context, taskARN string, cluster string) {
	c.logger.
		WithField("taskARN", taskARN).
		Info("Stopping Fargate task")

	err := c.awsFargate.StopTask(ctx.Ctx, taskARN, cluster)
	if err != nil {
		c.logger.
			WithField("taskARN", taskARN).
			WithError(err).
			Error("Error during stop task")
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
