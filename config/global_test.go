package config

import (
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
		Cluster:         "cluster-name",
		Region:          "us-east-1",
		TaskDefinition:  "my-task-definition:1",
		Subnet:          "subnet-XYZ",
		SecurityGroup:   "sg-XYZ",
		EnablePublicIP:  true,
		UsePublicIP:     false,
		PlatformVersion: "1.4.0",
		TaskWaitConfig:  TaskWaitConfig{100, 6},
	}
	expectedArrayConfig := Fargate{
		Cluster:         "cluster-name",
		Region:          "us-east-1",
		TaskDefinition:  "my-task-definition:1",
		Subnet:          "subnet-WXY,subnet-XYZ",
		SecurityGroup:   "sg-WXY,sg-XYZ",
		EnablePublicIP:  true,
		UsePublicIP:     false,
		PlatformVersion: "1.4.0",
		TaskWaitConfig:  TaskWaitConfig{100, 6},
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
		// "fail when securitygroups limit exceeded": {
		// 	filename:               "testdata/invalid-security-groups-limit.toml",
		// 	want:                   Fargate{},
		// 	subnetsCount:           0,
		// 	securityGroupsCount:    0,
		// 	expectedSubnets:        splitStringToStringArray(""),
		// 	expectedSecurityGroups: splitStringToStringArray(""),
		// 	expectError:            errors.New("A maximum of 5 security groups can be specified for awsvpcConfiguration."),
		// },
		// "fail when cluster omitted": {
		// 	filename:    "testdata/invalid-config-no-cluser.toml",
		// 	want:        Fargate{},
		// 	expectError: errors.New("Cluster name not specified in configuration file."),
		// },
		// "fail when region omitted": {
		// 	filename:    "testdata/invalid-config-no-region.toml",
		// 	want:        Fargate{},
		// 	expectError: errors.New("Region not specified in configuration file."),
		// },
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cfg, err := LoadFromFile(test.filename)

			assert.Equal(t, test.expectError, err)
			assert.NotNil(t, cfg)
			//cfg.Fargate.Subnets()
			assert.Equal(t, test.want, cfg.Fargate)
			assert.Equal(t, test.subnetsCount, len(cfg.Fargate.Subnets()))
			assert.Equal(t, test.securityGroupsCount, len(cfg.Fargate.SecurityGroups()))
			assert.Equal(t, test.expectedSubnets, cfg.Fargate.Subnets())
			assert.Equal(t, test.expectedSecurityGroups, cfg.Fargate.SecurityGroups())
		})
	}
}

func splitStringToStringArray(s string) []*string {
	return toPointerSlice(strings.Fields(strings.ReplaceAll(s, ",", " ")))
}
