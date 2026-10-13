package config

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	logging "gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
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
		TaskHealthWaitConfig: TaskHealthWaitConfig{3, time.Duration(30) * time.Second},
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
		TaskHealthWaitConfig: TaskHealthWaitConfig{3, time.Duration(30) * time.Second},
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

func Test_AssertRequiredConfigInvalidValues(t *testing.T) {
	expectedGlobal := getGlobalConfiguration(
		TaskHealthWaitConfig{
			Retries:       AWSWaitTaskHealthDefaultRetries,
			CheckInterval: AWSWaitTaskHealthDefaultCheckInterval,
		},
		TaskWaitConfig{
			MaxAttempts: AWSWaitDefaultMaxAttempts,
			WaitTimeout: AWSWaitDefaultTimeoutSeconds,
		})
	tests := map[string]struct {
		global         Global
		expectedGlobal Global
	}{
		"Invalid TaskWaitConfig.MaxAttempts": {
			global: getGlobalConfiguration(
				TaskHealthWaitConfig{
					Retries:       AWSWaitTaskHealthDefaultRetries,
					CheckInterval: AWSWaitTaskHealthDefaultCheckInterval,
				},
				TaskWaitConfig{
					MaxAttempts: 0,
					WaitTimeout: AWSWaitDefaultTimeoutSeconds,
				}),
			expectedGlobal: expectedGlobal,
		},
		"Invalid TaskWaitConfig.WaitTimeout": {
			global: getGlobalConfiguration(
				TaskHealthWaitConfig{
					Retries:       AWSWaitTaskHealthDefaultRetries,
					CheckInterval: AWSWaitTaskHealthDefaultCheckInterval,
				},
				TaskWaitConfig{
					MaxAttempts: AWSWaitDefaultMaxAttempts,
					WaitTimeout: 0,
				}),
			expectedGlobal: expectedGlobal,
		},
		"Invalid TaskHealthWaitConfig.Retries": {
			global: getGlobalConfiguration(
				TaskHealthWaitConfig{
					Retries:       0,
					CheckInterval: AWSWaitTaskHealthDefaultCheckInterval,
				},
				TaskWaitConfig{
					MaxAttempts: AWSWaitDefaultMaxAttempts,
					WaitTimeout: AWSWaitDefaultTimeoutSeconds,
				}),
			expectedGlobal: expectedGlobal,
		},
		"Invalid TaskHealthWaitConfig.CheckInterval": {
			global: getGlobalConfiguration(
				TaskHealthWaitConfig{
					Retries:       AWSWaitTaskHealthDefaultRetries,
					CheckInterval: time.Millisecond,
				},
				TaskWaitConfig{
					MaxAttempts: AWSWaitDefaultMaxAttempts,
					WaitTimeout: AWSWaitDefaultTimeoutSeconds,
				}),
			expectedGlobal: expectedGlobal,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := test.global
			err := cfg.AssertRequiredConfig(logging.NewNullLogger())
			assert.NoError(t, err)
			assert.Equal(t, cfg, test.expectedGlobal)
		})
	}
}

func Test_AssertRequiredConfigDefaultsEachInvalidWaitSettingOnItsOwn(t *testing.T) {
	validHealth := TaskHealthWaitConfig{Retries: 5, CheckInterval: 10 * time.Second}
	validWait := TaskWaitConfig{MaxAttempts: 50, WaitTimeout: 7}

	tests := map[string]struct {
		health       TaskHealthWaitConfig
		wait         TaskWaitConfig
		wantHealth   TaskHealthWaitConfig
		wantWait     TaskWaitConfig
		wantWarnings []string
	}{
		"all wait settings valid": {
			health:     validHealth,
			wait:       validWait,
			wantHealth: validHealth,
			wantWait:   validWait,
		},
		"invalid TaskWaitConfig.MaxAttempts": {
			health:       validHealth,
			wait:         TaskWaitConfig{MaxAttempts: 0, WaitTimeout: 7},
			wantHealth:   validHealth,
			wantWait:     TaskWaitConfig{MaxAttempts: AWSWaitDefaultMaxAttempts, WaitTimeout: 7},
			wantWarnings: []string{"Invalid number of TaskWaitConfig.MaxAttempts specified: 0. Defaulting to 100."},
		},
		"invalid TaskWaitConfig.WaitTimeout": {
			health:       validHealth,
			wait:         TaskWaitConfig{MaxAttempts: 50, WaitTimeout: 0},
			wantHealth:   validHealth,
			wantWait:     TaskWaitConfig{MaxAttempts: 50, WaitTimeout: AWSWaitDefaultTimeoutSeconds},
			wantWarnings: []string{"Invalid TaskWaitConfig.WaitTimeout specified: 0. Defaulting to 6."},
		},
		"invalid TaskHealthWaitConfig.Retries": {
			health:       TaskHealthWaitConfig{Retries: 0, CheckInterval: 10 * time.Second},
			wait:         validWait,
			wantHealth:   TaskHealthWaitConfig{Retries: AWSWaitTaskHealthDefaultRetries, CheckInterval: 10 * time.Second},
			wantWait:     validWait,
			wantWarnings: []string{"Invalid number of TaskHealthWaitConfig.Retries specified: 0. Defaulting to 3."},
		},
		"invalid TaskHealthWaitConfig.CheckInterval": {
			health:       TaskHealthWaitConfig{Retries: 5, CheckInterval: 0},
			wait:         validWait,
			wantHealth:   TaskHealthWaitConfig{Retries: 5, CheckInterval: AWSWaitTaskHealthDefaultCheckInterval},
			wantWait:     validWait,
			wantWarnings: []string{"Invalid TaskHealthWaitConfig.CheckInterval specified: 0s. Defaulting to 30s."},
		},
		"invalid TaskWaitConfig.WaitTimeout and TaskHealthWaitConfig.Retries": {
			health:     TaskHealthWaitConfig{Retries: -2, CheckInterval: 10 * time.Second},
			wait:       TaskWaitConfig{MaxAttempts: 50, WaitTimeout: -1},
			wantHealth: TaskHealthWaitConfig{Retries: AWSWaitTaskHealthDefaultRetries, CheckInterval: 10 * time.Second},
			wantWait:   TaskWaitConfig{MaxAttempts: 50, WaitTimeout: AWSWaitDefaultTimeoutSeconds},
			wantWarnings: []string{
				"Invalid TaskWaitConfig.WaitTimeout specified: -1. Defaulting to 6.",
				"Invalid number of TaskHealthWaitConfig.Retries specified: -2. Defaulting to 3.",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logger, output := logging.NewBufferedLogger()
			cfg := getGlobalConfiguration(test.health, test.wait)

			err := cfg.AssertRequiredConfig(logger)

			require.NoError(t, err)
			assert.Equal(t, test.wantHealth, cfg.Fargate.TaskHealthWaitConfig)
			assert.Equal(t, test.wantWait, cfg.Fargate.TaskWaitConfig)

			logged := output.String()
			for _, warning := range test.wantWarnings {
				assert.Contains(t, logged, warning)
			}
			assert.Equal(t, len(test.wantWarnings), strings.Count(logged, "level=warning"), logged)
			assert.NotContains(t, logged, "%!")
		})
	}
}

func Test_AssertRequiredConfig(t *testing.T) {
	tests := map[string]struct {
		global      Global
		expectError error
	}{
		"valid minimal config for awsvpc": {
			global: Global{"debug", "", "json", STSConfiguration{}, Fargate{Cluster: "mycluster", TaskDefinition: "task:arn", Region: "eu-west-1", Subnet: "subnet-xyz", SecurityGroup: "sg-123"}, TaskMetadata{}, SSH{}},
		},
		"cluster name not specified": {
			global:      Global{"debug", "", "json", STSConfiguration{}, Fargate{TaskDefinition: "task:arn", Region: "eu-west-1"}, TaskMetadata{}, SSH{}},
			expectError: errors.New("required cluster name not specified"),
		},
		"task definition not specified": {
			global:      Global{"debug", "", "json", STSConfiguration{}, Fargate{Cluster: "mycluster", Region: "eu-west-1"}, TaskMetadata{}, SSH{}},
			expectError: errors.New("required TaskDefinition arn not specified"),
		},
		"region not specified": {
			global:      Global{"debug", "", "json", STSConfiguration{}, Fargate{Cluster: "mycluster", TaskDefinition: "task:arn"}, TaskMetadata{}, SSH{}},
			expectError: errors.New("region not specified in configuration file"),
		},
		"enabled public api with enabling its use": {
			global:      Global{"debug", "", "json", STSConfiguration{}, Fargate{Cluster: "mycluster", TaskDefinition: "task:arn", Region: "eu-west-1", UsePublicIP: true}, TaskMetadata{}, SSH{}},
			expectError: errors.New("can't use public IP to communicate if EnablePublicIP is not enabled in configuration file"),
		},
		"at least one securitygroup should be specified for awsvpc": {
			global:      Global{"debug", "", "json", STSConfiguration{}, Fargate{Cluster: "mycluster", TaskDefinition: "task:arn", Region: "eu-west-1", Subnet: "subnet-xyz"}, TaskMetadata{}, SSH{}},
			expectError: errors.New("at leat 1 security group must be specified for awsvpcConfiguration"),
		},
		"at least one subnet should be specified for awsvpc": {
			global:      Global{"debug", "", "json", STSConfiguration{}, Fargate{Cluster: "mycluster", TaskDefinition: "task:arn", Region: "eu-west-1", SecurityGroup: "sg-1231"}, TaskMetadata{}, SSH{}},
			expectError: errors.New("at leat 1 subnet must be specified for awsvpcConfiguration"),
		},
		"too many securitygroups specified for awsvpc": {
			global:      Global{"debug", "", "json", STSConfiguration{}, Fargate{Cluster: "mycluster", TaskDefinition: "task:arn", Region: "eu-west-1", Subnet: "subnet-xyz", SecurityGroup: "sg-1231, sg-1232, sg-1233, sg-1234, sg-1235, sg-1236"}, TaskMetadata{}, SSH{}},
			expectError: errors.New("a maximum of 5 security groups can be specified for awsvpcConfiguration"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := test.global
			err := cfg.AssertRequiredConfig(logging.NewNullLogger())
			assert.Equal(t, test.expectError, err)
		})
	}
}

func splitStringToStringArray(s string) []*string {
	return toPointerSlice(strings.Fields(strings.ReplaceAll(s, ",", " ")))
}

func getGlobalConfiguration(taskHealthWaitConfig TaskHealthWaitConfig, taskWaitConfig TaskWaitConfig) Global {
	return Global{
		"debug", "", "json",
		STSConfiguration{},
		Fargate{
			Cluster:              "mycluster",
			TaskDefinition:       "task:arn",
			Region:               "eu-west-1",
			Subnet:               "subnet-xyz",
			TaskWaitConfig:       taskWaitConfig,
			TaskHealthWaitConfig: taskHealthWaitConfig,
			SecurityGroup:        "sg-123",
		},
		TaskMetadata{},
		SSH{},
	}
}
