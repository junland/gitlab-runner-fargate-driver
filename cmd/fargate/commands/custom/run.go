package custom

import (
	"errors"
	"fmt"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/cli"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
)

func NewRunCommand() cli.Command {
	cmd := new(RunCommand)
	cmd.abstractCustomCommand.customCommand = cmd

	return cli.Command{
		Handler: cmd,
		Config: cli.Config{
			Name:      "run",
			Aliases:   []string{"r"},
			Usage:     "Run the script from Custom Executor",
			ArgsUsage: "path_to_script stage_name",
			Description: `
This is the implementation of Run stage of the Custom Executor.

The command will execute several scripts (provided by the Runner) on the
AWS Fargate task created previously by the 'prepare' command.

The command gets two arguments:

- path_to_script - which contains the path to the script that should be executed,
- stage_name - which contains the name of the job stage that is being executed.

Details about how this command is used can be found at https://docs.gitlab.com/runner/executors/custom.html#run.`,
		},
	}
}

type RunCommand struct {
	abstractCustomCommand

	cfg    config.Global
	logger logging.Logger
}

func (c *RunCommand) CustomExecute(ctx *cli.Context) error {
	argsCnt := ctx.Cli.NArg()
	if argsCnt < 2 {
		return errors.New("missing required arguments")
	}

	err := c.init(ctx)
	if err != nil {
		return fmt.Errorf("couldn't initialize RunCommand: %w", err)
	}

	c.logger.Info("Executing the command")

	// ADD IMPLEMENTATION HERE

	return nil
}

func (c *RunCommand) init(ctx *cli.Context) error {
	c.cfg = ctx.Config()
	c.logger = ctx.
		Logger().
		WithFields(logging.Fields{
			"command": "run_exec",
			"stage":   ctx.Cli.Args().Get(1),
		})

	// ADDITIONAL INITIALIZATION HERE

	return nil
}
