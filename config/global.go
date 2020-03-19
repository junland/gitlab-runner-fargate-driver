package config

import (
	"fmt"
	"io/ioutil"

	"github.com/BurntSushi/toml"
)

type Global struct {
	LogLevel  string
	LogFile   string
	LogFormat string

	FargateConfig FargateConfig
	TaskMetadata  TaskMetadata
	SSHConfig     SSHConfig
}

type FargateConfig struct {
	Cluster        string
	Region         string
	Subnet         string
	SecurityGroup  string
	TaskDefinition string
	EnablePublicIP bool
}

type TaskMetadata struct {
	Directory string
}

type SSHConfig struct {
	Username       string
	PrivateKeyPath string
}

func LoadFromFile(file string) (Global, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return Global{}, fmt.Errorf("couldn't read configuration file %q: %w", file, err)
	}

	var cfg Global

	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		return Global{}, fmt.Errorf("couldn't parse TOML content of the configuration file: %w", err)
	}

	return cfg, nil
}
