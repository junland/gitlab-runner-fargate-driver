package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/cli"
)

func TestLoadCliArgsEnvVars(t *testing.T) {
	originalTaskDefinition := "default-task-def:1"
	originalPlatformVersion := "LATEST"
	originalSubnet := "subnet-1"
	originalSecurityGroup := "sg-123456"
	overrideTaskDefinition := "another-task-def:1"
	overridePlatformVersion := "1.4.0"
	overrideSubnets := "subnet-1"         //,subnet-2"
	overrideSecurityGroups := "sg-123456" //,sg-654321"
	expectedOverrideSubnets := overrideSubnets
	expectedOverrideSecurityGroups := overrideSecurityGroups

	tests := map[string]struct {
		taskDefinitionOverrideValue  string
		platformVersionOverrideValue string
		subnetOverrideValue          string
		securityGroupOverrideValue   string
		expectedTaskDefinition       string
		expectedPlatformVersion      string
		expectedSubnet               string
		expectedSecurityGroup        string
	}{
		"Should keep original values if nothing received by command line or env variable": {
			taskDefinitionOverrideValue:  "",
			platformVersionOverrideValue: "",
			subnetOverrideValue:          "",
			securityGroupOverrideValue:   "",
			expectedTaskDefinition:       originalTaskDefinition,
			expectedPlatformVersion:      originalPlatformVersion,
			expectedSubnet:               originalSubnet,
			expectedSecurityGroup:        originalSecurityGroup,
		},
		"Should override task definition if received by command line or env variable": {
			taskDefinitionOverrideValue: overrideTaskDefinition,
			expectedTaskDefinition:      overrideTaskDefinition,
			expectedPlatformVersion:     originalPlatformVersion,
			expectedSubnet:              originalSubnet,
			expectedSecurityGroup:       originalSecurityGroup,
		},
		"Should override platform version if received by command line or env variable": {
			platformVersionOverrideValue: overridePlatformVersion,
			expectedTaskDefinition:       originalTaskDefinition,
			expectedPlatformVersion:      overridePlatformVersion,
			expectedSubnet:               originalSubnet,
			expectedSecurityGroup:        originalSecurityGroup,
		},
		"Should override subnets if received by command line or env variable": {
			subnetOverrideValue:     overrideSubnets,
			expectedSubnet:          expectedOverrideSubnets,
			expectedTaskDefinition:  originalTaskDefinition,
			expectedPlatformVersion: originalPlatformVersion,
			expectedSecurityGroup:   originalSecurityGroup,
		},
		"Should override security groups if received by command line or env variable": {
			securityGroupOverrideValue: overrideSecurityGroups,
			expectedSecurityGroup:      expectedOverrideSecurityGroups,
			expectedTaskDefinition:     originalTaskDefinition,
			expectedPlatformVersion:    originalPlatformVersion,
			expectedSubnet:             originalSubnet,
		},
		"Should override platform version and task definition if received by command line or env variable": {
			taskDefinitionOverrideValue:  overrideTaskDefinition,
			platformVersionOverrideValue: overridePlatformVersion,
			expectedTaskDefinition:       overrideTaskDefinition,
			expectedPlatformVersion:      overridePlatformVersion,
			expectedSubnet:               originalSubnet,
			expectedSecurityGroup:        originalSecurityGroup,
		},
		"Should override subnets and security groups if received by command line or env variable": {
			subnetOverrideValue:        overrideSubnets,
			securityGroupOverrideValue: overrideSecurityGroups,
			expectedSubnet:             expectedOverrideSubnets,
			expectedSecurityGroup:      expectedOverrideSecurityGroups,
			expectedTaskDefinition:     originalTaskDefinition,
			expectedPlatformVersion:    originalPlatformVersion,
		},
		"Should override all possible params if received by command line or env variable": {
			taskDefinitionOverrideValue:  overrideTaskDefinition,
			platformVersionOverrideValue: overridePlatformVersion,
			subnetOverrideValue:          overrideSubnets,
			securityGroupOverrideValue:   overrideSecurityGroups,
			expectedSubnet:               expectedOverrideSubnets,
			expectedSecurityGroup:        expectedOverrideSecurityGroups,
			expectedTaskDefinition:       overrideTaskDefinition,
			expectedPlatformVersion:      overridePlatformVersion,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			oldTaskDefGlobalValue := global.TaskDefinition
			oldPlatformVersionGlobalValue := global.PlatformVersion
			oldSubnetGlobalValue := global.Subnet
			oldSecurityGroupGlobalValue := global.SecurityGroup
			global.TaskDefinition = tt.taskDefinitionOverrideValue
			global.PlatformVersion = tt.platformVersionOverrideValue
			global.Subnet = tt.subnetOverrideValue
			global.SecurityGroup = tt.securityGroupOverrideValue

			defer func() {
				global.TaskDefinition = oldTaskDefGlobalValue
				global.PlatformVersion = oldPlatformVersionGlobalValue
				global.Subnet = oldSubnetGlobalValue
				global.SecurityGroup = oldSecurityGroupGlobalValue
			}()

			testContext := createCliContextForTests(originalTaskDefinition, originalPlatformVersion, originalSubnet, originalSecurityGroup)
			assert.Equal(t, originalTaskDefinition, testContext.Config().Fargate.TaskDefinition)
			assert.Equal(t, originalPlatformVersion, testContext.Config().Fargate.PlatformVersion)
			assert.Equal(t, originalSubnet, testContext.Config().Fargate.Subnet)
			assert.Equal(t, originalSecurityGroup, testContext.Config().Fargate.SecurityGroup)

			err := loadCliArgsEnvVars(testContext)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedTaskDefinition, testContext.Config().Fargate.TaskDefinition)
			assert.Equal(t, tt.expectedPlatformVersion, testContext.Config().Fargate.PlatformVersion)
			assert.Equal(t, tt.expectedSubnet, testContext.Config().Fargate.Subnet)
			assert.Equal(t, tt.expectedSecurityGroup, testContext.Config().Fargate.SecurityGroup)
		})
	}
}

func createCliContextForTests(taskDefinition string, platformVersion string, subnet string, securityGroup string) *cli.Context {
	cliCtx := new(cli.Context)
	cliCtx.SetConfig(config.Global{
		Fargate: config.Fargate{
			TaskDefinition:  taskDefinition,
			PlatformVersion: platformVersion,
			Subnet:          subnet,
			SecurityGroup:   securityGroup,
		},
	})

	return cliCtx
}
