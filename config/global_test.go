package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_LoadWaitConfigFromFile(t *testing.T) {
	tests := map[string]struct {
		filename string
		want     TaskWaitConfig
	}{
		"wait config set to zero": {
			filename: "testdata/waitconfig-zero.toml",
			want:     TaskWaitConfig{0, 0},
		},
		"wait config not set": {
			filename: "testdata/no-waitconfig.toml",
			want:     TaskWaitConfig{100, 6},
		},
		"wait config non-default values are used": {
			filename: "testdata/waitconfig-non-default.toml",
			want:     TaskWaitConfig{50, 7},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cfg, err := LoadFromFile(test.filename)
			assert.NoError(t, err)
			assert.NotNil(t, cfg)
			assert.Equal(t, test.want, cfg.Fargate.TaskWaitConfig)
		})
	}
}

func Test_LoadSubnetConfigFromFile(t *testing.T) {
	expectedConfig := Fargate{
		Cluster:         "cluster-name",
		Region:          "us-east-1",
		TaskDefinition:  "my-task-definition:1",
		Subnets:         []string{"subnet-XYZ"},
		SecurityGroups:  []string{"sg-XYZ"},
		EnablePublicIP:  true,
		UsePublicIP:     false,
		PlatformVersion: "1.4.0",
		TaskWaitConfig:  TaskWaitConfig{100, 6},
	}

	tests := map[string]struct {
		filename    string
		want        Fargate
		expectError error
	}{
		"fail on both subnet properties specified": {
			filename:    "testdata/duplicate-subnet-config.toml",
			want:        Fargate{},
			expectError: errors.New("can't use both Subnet and Subnets configuration. Please only specify Subnets instead of Subnet."),
		},
		"fail on both securitygroup properties specified": {
			filename:    "testdata/duplicate-sg-config.toml",
			want:        Fargate{},
			expectError: errors.New("can't use both SecurityGroup and SecurityGroups configuration. Please only specify SecurityGroups instead of SecurityGroup."),
		},
		"expect subnets when legacy subnet specified": {
			filename:    "testdata/legacy-subnet-config.toml",
			want:        expectedConfig,
			expectError: nil,
		},
		"expect securitygroups when legacy securitygroup specified": {
			filename:    "testdata/legacy-securitygroup-config.toml",
			want:        expectedConfig,
			expectError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cfg, err := LoadFromFile(test.filename)

			assert.Equal(t, test.expectError, err)
			assert.NotNil(t, cfg)
			assert.Equal(t, test.want, cfg.Fargate)
		})
	}
}
