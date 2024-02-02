package main

import (
	"context"
	"os"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/cmd/fargate/commands/custom"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/cli"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/storage"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/runner"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/signal"
)

const (
	defaultConfigFile = "config.toml"
)

type globalFlags struct {
	Debug     bool   `long:"debug" description:"Set debug log level"`
	LogLevel  string `long:"log-level" description:"Set custom log level (debug, info, warning, error, fatal, panic)"`
	LogFile   string `long:"log-file" description:"File where logs should be saved"`
	LogFormat string `long:"log-format" description:"Format of log (text, json)"`

	ConfigFile string `long:"config" description:"Path to configuration file"`

	EvaluateTaskHealth *bool  `long:"evaluate-task-health" description:"Whether to evaluate task health" env:"CUSTOM_ENV_FARGATE_EVALUATE_TASK_HEALTH"`
	TaskDefinition     string `long:"task-def" description:"Task definition" env:"CUSTOM_ENV_FARGATE_TASK_DEFINITION"`
	PlatformVersion    string `long:"platform-version" description:"Fargate platform version" env:"CUSTOM_ENV_FARGATE_PLATFORM_VERSION"`
	Subnet             string `long:"subnet" description:"Task level subnets" env:"CUSTOM_ENV_FARGATE_SUBNET"`
	SecurityGroup      string `long:"security-group" description:"Task level security groups" env:"CUSTOM_ENV_FARGATE_SECURITY_GROUP"`

	RoleARN     string `long:"role-arn" description:"Role to assume with STS" env:"CUSTOM_ENV_ROLE_ARN"`
	SessionVars string `long:"session-vars" description:"Comma separated list of env variables to add to STS session as request tags" env:"CUSTOM_ENV_SESSION_VARS"`
}

var (
	global = &globalFlags{
		ConfigFile: defaultConfigFile,
	}

	closeLogFile = cli.NewNopHook()
)

func main() {
	logger := logging.New()

	ctx := startSignalHandler(logger)

	a := setUpApplication(ctx, logger)

	err := a.Run(os.Args)
	if err != nil {
		logger.
			WithError(err).
			Error("Application execution failed")

		runner.GetAdapter().GenerateExitFromError(err)
	}
}

func startSignalHandler(logger logging.Logger) context.Context {
	terminationHandler := signal.NewTerminationHandler(logger)
	go terminationHandler.HandleSignals()

	return terminationHandler.Context()
}

func setUpApplication(ctx context.Context, logger logging.Logger) *cli.App {
	a := cli.New(ctx, fargate.NAME, "Fargate driver for GitLab Runner's Custom Executor")

	a.AddBeforeFunc(func(ctx *cli.Context) error {
		ctx.SetLogger(logger)

		return nil
	})
	a.AddBeforeFunc(loadConfigurationFile)
	a.AddBeforeFunc(loadCliArgsEnvVars)
	a.AddBeforeFunc(assertRequiredConfig)
	a.AddBeforeFunc(updateLogLevel)
	a.AddBeforeFunc(updateLogFormat)
	a.AddBeforeFunc(setLoggingToFile)
	a.AddBeforeFunc(logStartupMessage)

	// If logging to file will be set, closeLogFile() will close the
	// used file. Otherwise tis a NOP call.
	a.AddAfterFunc(closeLogFile)

	a.AddGlobalFlagsFromStruct(global)

	a.RegisterCategory(custom.NewCustomCategory())

	return a
}

func logStartupMessage(ctx *cli.Context) error {
	ctx.
		Logger().
		WithFields(logging.Fields{
			"version": fargate.Version().ShortLine(),
		}).
		Infof("Starting %s", fargate.NAME)

	return nil
}

func loadConfigurationFile(ctx *cli.Context) error {
	cfg, err := config.LoadFromFile(global.ConfigFile)
	if err != nil {
		return err
	}

	ctx.SetConfig(cfg)

	return nil
}

//nolint:gocyclo
func loadCliArgsEnvVars(ctx *cli.Context) error {
	// Override parameters from config.toml if received by command line or env variable
	config := ctx.Config()

	if global.TaskDefinition != "" {
		config.Fargate.TaskDefinition = global.TaskDefinition
	}

	if global.PlatformVersion != "" {
		config.Fargate.PlatformVersion = global.PlatformVersion
	}

	if global.Subnet != "" {
		config.Fargate.Subnet = global.Subnet
	}

	if global.SecurityGroup != "" {
		config.Fargate.SecurityGroup = global.SecurityGroup
	}

	if global.EvaluateTaskHealth != nil {
		config.Fargate.EvaluateTaskHealth = *global.EvaluateTaskHealth
	}

	if global.RoleARN != "" {
		config.STSConfiguration.RoleARN = global.RoleARN
	}

	if global.SessionVars != "" {
		config.STSConfiguration.SessionVars = global.SessionVars
	}

	ctx.SetConfig(config)

	return nil
}

func assertRequiredConfig(ctx *cli.Context) error {
	// once we loaded the config from the config file and the cli/ENV
	// we now assert the config for required values to ensure we can
	// safely start the driver
	cfg := ctx.Config()
	return cfg.AssertRequiredConfig(ctx.Logger())
}

func updateLogLevel(ctx *cli.Context) error {
	logLevel := ctx.Config().LogLevel
	if global.Debug {
		logLevel = "debug"
	} else if global.LogLevel != "" {
		logLevel = global.LogLevel
	}

	if logLevel == "" {
		return nil
	}

	return ctx.Logger().SetLevel(logLevel)
}

func updateLogFormat(ctx *cli.Context) error {
	logFormat := ctx.Config().LogFormat
	if global.LogFormat != "" {
		logFormat = global.LogFormat
	}

	if logFormat == "" {
		return nil
	}

	return ctx.Logger().SetFormat(logFormat)
}

func setLoggingToFile(ctx *cli.Context) error {
	logFile := ctx.Config().LogFile
	if global.LogFile != "" {
		logFile = global.LogFile
	}

	if logFile == "" {
		return nil
	}

	logStorage := storage.NewFile(logFile)
	err := logStorage.Open()
	if err != nil {
		return err
	}

	closeLogFile = func(ctx *cli.Context) error {
		return logStorage.Close()
	}

	ctx.Logger().SetOutput(logStorage)

	return nil
}
