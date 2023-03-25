package config

import (
	"fmt"
	"os"

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
	// [Depracated]: Defines the subnet the container will run in
	Subnet string
	// [Depracated]: Defines the security group to be used by the container
	SecurityGroup  string
	Subnets        []string
	SecurityGroups []string
	TaskDefinition string
	// PropagateTags indicates whether to propagate the tags from the task definition to the running task
	PropagateTags  bool
	TaskWaitConfig TaskWaitConfig
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

	if len(cfg.Fargate.Cluster) < 1 {
		return Global{}, fmt.Errorf("Cluster name not specified in configuration file.")
	}

	if len(cfg.Fargate.Region) < 1 {
		// this driver should perhaps assume the region the runner is deployed to when no region is specified.
		return Global{}, fmt.Errorf("Region not specified in configuration file.")
	}

	if !cfg.Fargate.EnablePublicIP && cfg.Fargate.UsePublicIP {
		return Global{}, fmt.Errorf("can't use public IP to communicate if EnablePublicIP is not enabled in configuration file: %w", err)
	}

	if len(cfg.Fargate.Subnet) > 0 && len(cfg.Fargate.Subnets) > 0 {
		return Global{}, fmt.Errorf("can't use both Subnet and Subnets configuration. Please only specify Subnets instead of Subnet.")
	}

	if len(cfg.Fargate.SecurityGroup) > 0 && len(cfg.Fargate.SecurityGroups) > 0 {
		return Global{}, fmt.Errorf("can't use both SecurityGroup and SecurityGroups configuration. Please only specify SecurityGroups instead of SecurityGroup.")
	}

	// while we allow for backward compatibility, we assign Subnet and SecurityGroup to the array so we only use Subnets and SecurityGroup going forward
	// this will allow us to easily remove the singular properties in a future version
	if len(cfg.Fargate.Subnet) > 0 && len(cfg.Fargate.Subnets) == 0 {
		cfg.Fargate.Subnets = append(cfg.Fargate.Subnets, cfg.Fargate.Subnet)
		cfg.Fargate.Subnet = ""
	}

	if len(cfg.Fargate.SecurityGroup) > 0 && len(cfg.Fargate.SecurityGroups) == 0 {
		cfg.Fargate.SecurityGroups = append(cfg.Fargate.SecurityGroups, cfg.Fargate.SecurityGroup)
		cfg.Fargate.SecurityGroup = ""
	}

	if len(cfg.Fargate.SecurityGroups) > 5 {
		return Global{}, fmt.Errorf("A maximum of 5 security groups can be specified for awsvpcConfiguration.")
	}

	return cfg, nil
}
