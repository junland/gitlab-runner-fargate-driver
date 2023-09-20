package config

import (
	"errors"
	"strings"
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
		Cluster:              "cluster-name",
		Region:               "us-east-1",
		TaskDefinition:       "my-task-definition:1",
		Subnet:               "subnet-XYZ",
		SecurityGroup:        "sg-XYZ",
		EnablePublicIP:       true,
		UsePublicIP:          false,
		PlatformVersion:      "1.4.0",
		EvaluateTaskHealth:   false,
		TaskWaitConfig:       TaskWaitConfig{100, 6},
		TaskHealthWaitConfig: TaskWaitConfig{100, 6},
	}
	expectedArrayConfig := Fargate{
		Cluster:              "cluster-name",
		Region:               "us-east-1",
		TaskDefinition:       "my-task-definition:1",
		Subnet:               "subnet-WXY,subnet-XYZ",
		SecurityGroup:        "sg-WXY,sg-XYZ",
		EnablePublicIP:       true,
		UsePublicIP:          false,
		PlatformVersion:      "1.4.0",
		EvaluateTaskHealth:   false,
		TaskWaitConfig:       TaskWaitConfig{100, 6},
		TaskHealthWaitConfig: TaskWaitConfig{100, 6},
	}

	tests := map[string]struct {
		filename               string
		want                   Fargate
		subnetsCount           int
		securityGroupsCount    int
		expectedSubnets        []*string
		expectedSecurityGroups []*string
		expectError            error
	}{
		"expect subnets securitygroups return one value": {
			filename:               "testdata/legacy-subnet-config.toml",
			want:                   expectedConfig,
			subnetsCount:           1,
			securityGroupsCount:    1,
			expectedSubnets:        splitStringToStringArray(expectedConfig.Subnet),
			expectedSecurityGroups: splitStringToStringArray(expectedConfig.SecurityGroup),
			expectError:            nil,
		},
		"expect multiple when multiple specified": {
			filename:               "testdata/legacy-subnet-array-config.toml",
			want:                   expectedArrayConfig,
			subnetsCount:           2,
			securityGroupsCount:    2,
			expectedSubnets:        splitStringToStringArray(expectedArrayConfig.Subnet),
			expectedSecurityGroups: splitStringToStringArray(expectedArrayConfig.SecurityGroup),
			expectError:            nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cfg, err := LoadFromFile(test.filename)

			assert.Equal(t, test.expectError, err)
			assert.NotNil(t, cfg)
			assert.Equal(t, test.want, cfg.Fargate)
			assert.Equal(t, test.subnetsCount, len(cfg.Fargate.Subnets()))
			assert.Equal(t, test.securityGroupsCount, len(cfg.Fargate.SecurityGroups()))
			assert.Equal(t, test.expectedSubnets, cfg.Fargate.Subnets())
			assert.Equal(t, test.expectedSecurityGroups, cfg.Fargate.SecurityGroups())
		})
	}
}

func Test_AssertRequiredConfig(t *testing.T) {
	tests := map[string]struct {
		global      Global
		expectError error
	}{
		"valid minimal config for awsvpc": {
			global: Global{"debug", "", "json", Fargate{Cluster: "mycluster", TaskDefinition: "task:arn", Region: "eu-west-1", Subnet: "subnet-xyz", SecurityGroup: "sg-123"}, TaskMetadata{}, SSH{}},
		},
		"cluster name not specified": {
			global:      Global{"debug", "", "json", Fargate{TaskDefinition: "task:arn", Region: "eu-west-1"}, TaskMetadata{}, SSH{}},
			expectError: errors.New("required cluster name not specified"),
		},
		"task definition not specified": {
			global:      Global{"debug", "", "json", Fargate{Cluster: "mycluster", Region: "eu-west-1"}, TaskMetadata{}, SSH{}},
			expectError: errors.New("required TaskDefinition arn not specified"),
		},
		"region not specified": {
			global:      Global{"debug", "", "json", Fargate{Cluster: "mycluster", TaskDefinition: "task:arn"}, TaskMetadata{}, SSH{}},
			expectError: errors.New("region not specified in configuration file"),
		},
		"enabled public api with enabling its use": {
			global:      Global{"debug", "", "json", Fargate{Cluster: "mycluster", TaskDefinition: "task:arn", Region: "eu-west-1", UsePublicIP: true}, TaskMetadata{}, SSH{}},
			expectError: errors.New("can't use public IP to communicate if EnablePublicIP is not enabled in configuration file"),
		},
		"at least one securitygroup should be specified for awsvpc": {
			global:      Global{"debug", "", "json", Fargate{Cluster: "mycluster", TaskDefinition: "task:arn", Region: "eu-west-1", Subnet: "subnet-xyz"}, TaskMetadata{}, SSH{}},
			expectError: errors.New("at leat 1 security group must be specified for awsvpcConfiguration"),
		},
		"at least one subnet should be specified for awsvpc": {
			global:      Global{"debug", "", "json", Fargate{Cluster: "mycluster", TaskDefinition: "task:arn", Region: "eu-west-1", SecurityGroup: "sg-1231"}, TaskMetadata{}, SSH{}},
			expectError: errors.New("at leat 1 subnet must be specified for awsvpcConfiguration"),
		},
		"too many securitygroups specified for awsvpc": {
			global:      Global{"debug", "", "json", Fargate{Cluster: "mycluster", TaskDefinition: "task:arn", Region: "eu-west-1", Subnet: "subnet-xyz", SecurityGroup: "sg-1231, sg-1232, sg-1233, sg-1234, sg-1235, sg-1236"}, TaskMetadata{}, SSH{}},
			expectError: errors.New("a maximum of 5 security groups can be specified for awsvpcConfiguration"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := test.global
			err := cfg.AssertRequiredConfig()
			assert.Equal(t, test.expectError, err)
		})
	}
}

func splitStringToStringArray(s string) []*string {
	return toPointerSlice(strings.Fields(strings.ReplaceAll(s, ",", " ")))
}
