package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
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
	PropagateTags  bool
	TaskWaitConfig TaskWaitConfig
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

const (
	AWSWaitDefaultMaxAttempts    = 100
	AWSWaitDefaultTimeoutSeconds = 6
)

func LoadFromFile(file string) (Global, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return Global{}, fmt.Errorf("couldn't read configuration file %q: %w", file, err)
	}

	cfg := Global{
		Fargate: Fargate{
			TaskWaitConfig: TaskWaitConfig{
				MaxAttempts: AWSWaitDefaultMaxAttempts,
				WaitTimeout: AWSWaitDefaultTimeoutSeconds,
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
func (g *Global) AssertRequiredConfig() error {
	if len(g.Fargate.Cluster) < 1 {
		return fmt.Errorf("required cluster name not specified")
	}

	if len(g.Fargate.TaskDefinition) < 1 {
		return fmt.Errorf("required TaskDefinition arn not specified")
	}

	if len(g.Fargate.Region) < 1 {
		// this driver should perhaps assume the region the runner is deployed to when no region is specified.
		return fmt.Errorf("region not specified in configuration file")
	}

	if !g.Fargate.EnablePublicIP && g.Fargate.UsePublicIP {
		return fmt.Errorf("can't use public IP to communicate if EnablePublicIP is not enabled in configuration file")
	}

	// we only care about the security group and subnet config if running under awsvpc network mode
	if len(g.Fargate.SecurityGroups()) < 1 {
		return fmt.Errorf("at leat 1 security group must be specified for awsvpcConfiguration")
	}

	if len(g.Fargate.Subnets()) < 1 {
		return fmt.Errorf("at leat 1 subnet must be specified for awsvpcConfiguration")
	}

	if len(g.Fargate.SecurityGroups()) > 5 {
		return fmt.Errorf("a maximum of 5 security groups can be specified for awsvpcConfiguration")
	}

	return nil
}
