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
		return fmt.Errorf("couldn't initialize PrepareCommand: %w", err)
	}

	c.logger.Info("Executing the command")

	taskARN, containerIP, err := c.prepareNewFargateTask(ctx)
	if err != nil {
		if taskARN != "" {
			c.logger.Error(
				"Error during task initialization. Will kill the task for cleanup",
			)
			c.awsFargate.StopTask(ctx.Ctx, taskARN, c.cfg.FargateConfig.Cluster)
		}

		return fmt.Errorf("error preparing new Fargate task: %w", err)
	}

	err = c.persistDataForLaterStages(taskARN, containerIP)
	if err != nil {
		c.logger.Error(
			`Error persiting task data necessary for next stages.
			Will kill the task for cleanup`,
		)
		c.awsFargate.StopTask(ctx.Ctx, taskARN, c.cfg.FargateConfig.Cluster)

		return fmt.Errorf("error persisting task ARN for later stages: %w", err)
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
		return fmt.Errorf("error initializing Fargate adapter: %w", err)
	}

	c.metadataManager = c.newMetadataManager(c.logger, c.cfg.TemporaryFilesDir)

	return nil
}

func (c *PrepareCommand) prepareNewFargateTask(ctx *cli.Context) (string, string, error) {
	c.logger.Info("Preparing Fargate task")

	taskDefinition := c.cfg.FargateConfig.TaskDefinition
	connection := aws.ConnectionSettings{
		Cluster:        c.cfg.FargateConfig.Cluster,
		Subnet:         c.cfg.FargateConfig.Subnet,
		SecurityGroup:  c.cfg.FargateConfig.SecurityGroup,
		EnablePublicIP: c.cfg.FargateConfig.EnablePublicIP,
	}

	// Start the Fargate task
	taskARN, err := c.awsFargate.RunTask(ctx.Ctx, taskDefinition, connection)
	if err != nil {
		return taskARN, "", fmt.Errorf("error starting Fargate task: %w", err)
	}

	// Wait the task to be in "running" state
	err = c.awsFargate.WaitUntilTaskRunning(ctx.Ctx, taskARN, connection.Cluster)
	if err != nil {
		return taskARN, "", fmt.Errorf("error waiting Fargate be in running state: %w", err)
	}

	// Get the container IP
	ip, err := c.awsFargate.GetContainerIP(ctx.Ctx, taskARN, connection.Cluster, connection.EnablePublicIP)
	if err != nil {
		return taskARN, ip, fmt.Errorf("error fetching the container IP: %w", err)
	}

	return taskARN, ip, nil
}

func (c *PrepareCommand) persistDataForLaterStages(taskARN string, containerIP string) error {
	c.logger.Info("Persisting data that will be used by other commands")

	data := task.Data{
		TaskARN:     taskARN,
		ContainerIP: containerIP,
	}
	err := c.metadataManager.Persist(data)
	if err != nil {
		return fmt.Errorf("error persisting data for next stages: %w", err)
	}

	return nil
}
