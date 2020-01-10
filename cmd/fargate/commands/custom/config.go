package custom

import (
	"fmt"
	"os"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/cli"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/runner"
)

func NewConfigCommand() cli.Command {
	cmd := new(ConfigCommand)
	cmd.abstractCustomCommand.customCommand = cmd

	return cli.Command{
		Handler: cmd,
		Config: cli.Config{
			Name:    "config",
			Aliases: []string{"co"},
			Usage:   "Provide some configuration details for Custom Executor",
			Description: `
This is the implementation of Config stage of the Custom Executor.

This command is providing some configuration details that can be next used
by Runner's Custom Executor.

This command requires GitLab Runner 12.4 or higher to work properly.

Details about how this command is used can be found at
https://docs.gitlab.com/runner/executors/custom.html#config.`,
		},
	}
}

type ConfigCommand struct {
	abstractCustomCommand

	cfg    config.Global
	logger logging.Logger
}

func (c *ConfigCommand) CustomExecute(ctx *cli.Context) error {
	err := c.init(ctx)
	if err != nil {
		return fmt.Errorf("couldn't initialize ConfigCommand: %w", err)
	}

	c.logger.Info("Executing the command")

	hostname := "shouldBeUniqueForBuild" // TODO: This needs to be changed! :)

	err = runner.GetAdapter().WriteCustomExecutorConfig(os.Stdout, hostname)
	if err != nil {
		return fmt.Errorf("couldn't write JSON output for the ConfigCommand: %w", err)
	}

	return nil
}

func (c *ConfigCommand) init(ctx *cli.Context) error {
	c.cfg = ctx.Config()
	c.logger = ctx.
		Logger().
		WithField("command", "config_exec")

	// ADDITIONAL INITIALIZATION HERE

	return nil
}
