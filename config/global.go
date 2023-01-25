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
	Subnet          string
	SecurityGroup   string
	TaskDefinition  string
	TaskWaitConfig  TaskWaitConfig
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

func (global *Global) fill_config_defaults() {

	// setting default values
	// if no values present
	if global.Fargate.TaskWaitConfig.MaxAttempts < 1 {
		global.Fargate.TaskWaitConfig.MaxAttempts = AWSWaitDefaultMaxAttempts
	}

	if global.Fargate.TaskWaitConfig.WaitTimeout < 1 {
		global.Fargate.TaskWaitConfig.WaitTimeout = AWSWaitDefaultTimeoutSeconds
	}
}

func LoadFromFile(file string) (Global, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return Global{}, fmt.Errorf("couldn't read configuration file %q: %w", file, err)
	}

	cfg := Global{
		Fargate: Fargate{
			TaskWaitConfig: TaskWaitConfig{
				MaxAttempts: 100,
				WaitTimeout: 6,
			},
		},
	}

	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		return Global{}, fmt.Errorf("couldn't parse TOML content of the configuration file: %w", err)
	}

	if !cfg.Fargate.EnablePublicIP && cfg.Fargate.UsePublicIP {
		return Global{}, fmt.Errorf("can't use public IP to communicate if EnablePublicIP is not enabled in configuration file: %w", err)
	}

	cfg.fill_config_defaults()

	return cfg, nil
}
