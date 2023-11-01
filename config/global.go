package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
)

type Global struct {
	LogLevel  string
	LogFile   string
	LogFormat string

	Fargate      Fargate
	TaskMetadata TaskMetadata
	SSH          SSH
}

type Fargate struct {
	Cluster         string
	EnablePublicIP  bool
	UsePublicIP     bool
	PlatformVersion string
	Region          string
	Subnet          string
	SecurityGroup   string
	TaskDefinition  string
	// PropagateTags indicates whether to propagate the tags from the task definition to the running task
	PropagateTags bool
	// EvaluateTaskHealth indicates whether to wait for the task to be healthy before attempting to run commands
	EvaluateTaskHealth   bool
	TaskWaitConfig       TaskWaitConfig
	TaskHealthWaitConfig TaskHealthWaitConfig
}

func (f *Fargate) Subnets() []*string {
	return toPointerSlice(strings.Fields(strings.ReplaceAll(f.Subnet, ",", " ")))
}

func (f *Fargate) SecurityGroups() []*string {
	return toPointerSlice(strings.Fields(strings.ReplaceAll(f.SecurityGroup, ",", " ")))
}

func toPointerSlice[T any](s []T) []*T {
	result := make([]*T, 0, len(s))
	for i := range s {
		result = append(result, &s[i])
	}
	return result
}

type TaskMetadata struct {
	Directory string
}

type SSH struct {
	Username string
	Port     int
}

type TaskWaitConfig struct {
	MaxAttempts int
	WaitTimeout int
}

type TaskHealthWaitConfig struct {
	Retries       int
	CheckInterval time.Duration
}

const (
	AWSWaitDefaultMaxAttempts    = 100
	AWSWaitDefaultTimeoutSeconds = 6
)

const (
	AWSWaitTaskHealthDefaultRetries       = 3
	AWSWaitTaskHealthDefaultCheckInterval = 30 * time.Second
)

func LoadFromFile(file string) (Global, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return Global{}, fmt.Errorf("couldn't read configuration file %q: %w", file, err)
	}

	cfg := Global{
		Fargate: Fargate{
			EvaluateTaskHealth: false,
			TaskWaitConfig: TaskWaitConfig{
				MaxAttempts: AWSWaitDefaultMaxAttempts,
				WaitTimeout: AWSWaitDefaultTimeoutSeconds,
			},
			TaskHealthWaitConfig: TaskHealthWaitConfig{
				Retries:       AWSWaitTaskHealthDefaultRetries,
				CheckInterval: AWSWaitTaskHealthDefaultCheckInterval,
			},
		},
	}

	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		return Global{}, fmt.Errorf("couldn't parse TOML content of the configuration file: %w", err)
	}

	return cfg, nil
}

//nolint:gocyclo
func (g *Global) AssertRequiredConfig(logger logging.Logger) error {
	switch {
	case len(g.Fargate.Cluster) < 1:
		return fmt.Errorf("required cluster name not specified")
	case len(g.Fargate.TaskDefinition) < 1:
		return fmt.Errorf("required TaskDefinition arn not specified")
	case len(g.Fargate.Region) < 1:
		// this driver should perhaps assume the region the runner is deployed to when no region is specified.
		return fmt.Errorf("region not specified in configuration file")
	case !g.Fargate.EnablePublicIP && g.Fargate.UsePublicIP:
		return fmt.Errorf("can't use public IP to communicate if EnablePublicIP is not enabled in configuration file")
	// we only care about the security group and subnet config if running under awsvpc network mode
	case len(g.Fargate.SecurityGroups()) < 1:
		return fmt.Errorf("at leat 1 security group must be specified for awsvpcConfiguration")
	case len(g.Fargate.Subnets()) < 1:
		return fmt.Errorf("at leat 1 subnet must be specified for awsvpcConfiguration")
	case len(g.Fargate.SecurityGroups()) > 5:
		return fmt.Errorf("a maximum of 5 security groups can be specified for awsvpcConfiguration")
	case g.Fargate.TaskWaitConfig.MaxAttempts < 1:
		logger.Warnf("Invalid number of TaskWaitConfig.MaxAttempts specified: %s. Defaulting to %s.", g.Fargate.TaskWaitConfig.MaxAttempts, AWSWaitDefaultMaxAttempts)
		g.Fargate.TaskWaitConfig.MaxAttempts = AWSWaitDefaultMaxAttempts
		fallthrough
	case g.Fargate.TaskWaitConfig.WaitTimeout < 1:
		logger.Warnf("Invalid TaskWaitConfig.WaitTimeout specified: %s. Defaulting to %s.", g.Fargate.TaskWaitConfig.WaitTimeout, AWSWaitDefaultTimeoutSeconds)
		g.Fargate.TaskWaitConfig.WaitTimeout = AWSWaitDefaultTimeoutSeconds
		fallthrough
	case g.Fargate.TaskHealthWaitConfig.Retries < 1:
		logger.Warnf("Invalid number of TaskHealthWaitConfig.Retries specified: %s. Defaulting to %s.", g.Fargate.TaskHealthWaitConfig.Retries, AWSWaitTaskHealthDefaultRetries)
		g.Fargate.TaskHealthWaitConfig.Retries = AWSWaitTaskHealthDefaultRetries
		fallthrough
	case g.Fargate.TaskHealthWaitConfig.CheckInterval < time.Second:
		logger.Warnf("Invalid TaskHealthWaitConfig.CheckInterval specified: %s. Defaulting to %s.", g.Fargate.TaskHealthWaitConfig.CheckInterval, AWSWaitTaskHealthDefaultCheckInterval)
		g.Fargate.TaskHealthWaitConfig.CheckInterval = AWSWaitTaskHealthDefaultCheckInterval
		return nil
	default:
		return nil
	}
}
