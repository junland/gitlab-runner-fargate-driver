package custom

import (
	"fmt"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/cli"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
)

func NewCleanupCommand() cli.Command {
	cmd := new(CleanupCommand)
	cmd.abstractCustomCommand.customCommand = cmd

	return cli.Command{
		Handler: cmd,
		Config: cli.Config{
			Name:    "cleanup",
			Aliases: []string{"cl"},
			Usage:   "Cleanup the environment for Custom Executor",
			Description: `
This is the implementation of Cleanup stage of the Custom Executor.

This command is removing the Virtual Machine that was dedicated for
the job scripts execution.

Details about how this command is used can be found at
https://docs.gitlab.com/runner/executors/custom.html#cleanup.`,
		},
	}
}

type CleanupCommand struct {
	abstractCustomCommand

	cfg    config.Global
	logger logging.Logger
}

func (c *CleanupCommand) CustomExecute(ctx *cli.Context) error {
	err := c.init(ctx)
	if err != nil {
		return fmt.Errorf("couldn't initialize CleanupCommand: %w", err)
	}

	c.logger.Info("Executing the command")

	// ADD IMPLEMENTATION HERE

	return nil
}

func (c *CleanupCommand) init(ctx *cli.Context) error {
	c.cfg = ctx.Config()
	c.logger = ctx.
		Logger().
		WithField("command", "cleanup_exec")

	// ADDITIONAL INITIALIZATION HERE

	return nil
}
