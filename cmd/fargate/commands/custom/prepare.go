package custom

import (
	"fmt"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/cli"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
)

func NewPrepareCommand() cli.Command {
	cmd := new(PrepareCommand)
	cmd.abstractCustomCommand.customCommand = cmd

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

type PrepareCommand struct {
	abstractCustomCommand

	cfg    config.Global
	logger logging.Logger
}

func (c *PrepareCommand) CustomExecute(ctx *cli.Context) error {
	err := c.init(ctx)
	if err != nil {
		return fmt.Errorf("couldn't initialize PrepareCommand: %w", err)
	}

	c.logger.Info("Executing the command")

	// ADD IMPLEMENTATION HERE

	return nil
}

func (c *PrepareCommand) init(ctx *cli.Context) error {
	c.cfg = ctx.Config()
	c.logger = ctx.
		Logger().
		WithField("command", "prepare_exec")

	// ADDITIONAL INITIALIZATION HERE

	return nil
}
