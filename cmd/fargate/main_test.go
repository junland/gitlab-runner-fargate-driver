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
	originalSubnets := []string{"subnet-1"}
	originalSecurityGroups := []string{"sg-123456"}
	overrideTaskDefinition := "another-task-def:1"
	overridePlatformVersion := "1.4.0"
	overrideSubnets := "subnet-1,subnet-2"
	overrideSecurityGroups := "sg-123456,sg-654321"
	expectedOverrideSubnets := splitStringToStringArray(overrideSubnets)
	expectedOverrideSecurityGroups := splitStringToStringArray(overrideSecurityGroups)

	tests := map[string]struct {
		taskDefinitionOverrideValue  string
		platformVersionOverrideValue string
		subnetsOverrideValue         string
		securityGroupsOverrideValue  string
		expectedTaskDefinition       string
		expectedPlatformVersion      string
		expectedSubnets              []string
		expectedSecurityGroups       []string
	}{
		"Should keep original values if nothing received by command line or env variable": {
			taskDefinitionOverrideValue:  "",
			platformVersionOverrideValue: "",
			subnetsOverrideValue:         "",
			securityGroupsOverrideValue:  "",
			expectedTaskDefinition:       originalTaskDefinition,
			expectedPlatformVersion:      originalPlatformVersion,
			expectedSubnets:              originalSubnets,
			expectedSecurityGroups:       originalSecurityGroups,
		},
		"Should override task definition if received by command line or env variable": {
			taskDefinitionOverrideValue: overrideTaskDefinition,
			expectedTaskDefinition:      overrideTaskDefinition,
			expectedPlatformVersion:     originalPlatformVersion,
			expectedSubnets:             originalSubnets,
			expectedSecurityGroups:      originalSecurityGroups,
		},
		"Should override platform version if received by command line or env variable": {
			platformVersionOverrideValue: overridePlatformVersion,
			expectedTaskDefinition:       originalTaskDefinition,
			expectedPlatformVersion:      overridePlatformVersion,
			expectedSubnets:              originalSubnets,
			expectedSecurityGroups:       originalSecurityGroups,
		},
		"Should override subnets if received by command line or env variable": {
			subnetsOverrideValue:    overrideSubnets,
			expectedSubnets:         expectedOverrideSubnets,
			expectedTaskDefinition:  originalTaskDefinition,
			expectedPlatformVersion: originalPlatformVersion,
			expectedSecurityGroups:  originalSecurityGroups,
		},
		"Should override security groups if received by command line or env variable": {
			securityGroupsOverrideValue: overrideSecurityGroups,
			expectedSecurityGroups:      expectedOverrideSecurityGroups,
			expectedTaskDefinition:      originalTaskDefinition,
			expectedPlatformVersion:     originalPlatformVersion,
			expectedSubnets:             originalSubnets,
		},
		"Should override platform version and task definition if received by command line or env variable": {
			taskDefinitionOverrideValue:  overrideTaskDefinition,
			platformVersionOverrideValue: overridePlatformVersion,
			expectedTaskDefinition:       overrideTaskDefinition,
			expectedPlatformVersion:      overridePlatformVersion,
			expectedSubnets:              originalSubnets,
			expectedSecurityGroups:       originalSecurityGroups,
		},
		"Should override subnets and security groups if received by command line or env variable": {
			subnetsOverrideValue:        overrideSubnets,
			securityGroupsOverrideValue: overrideSecurityGroups,
			expectedSubnets:             expectedOverrideSubnets,
			expectedSecurityGroups:      expectedOverrideSecurityGroups,
			expectedTaskDefinition:      originalTaskDefinition,
			expectedPlatformVersion:     originalPlatformVersion,
		},
		"Should override all possible params if received by command line or env variable": {
			taskDefinitionOverrideValue:  overrideTaskDefinition,
			platformVersionOverrideValue: overridePlatformVersion,
			subnetsOverrideValue:         overrideSubnets,
			securityGroupsOverrideValue:  overrideSecurityGroups,
			expectedSubnets:              expectedOverrideSubnets,
			expectedSecurityGroups:       expectedOverrideSecurityGroups,
			expectedTaskDefinition:       overrideTaskDefinition,
			expectedPlatformVersion:      overridePlatformVersion,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			oldTaskDefGlobalValue := global.TaskDefinition
			oldPlatformVersionGlobalValue := global.PlatformVersion
			oldSubnetsGlobalValue := global.Subnets
			oldSecurityGroupsGlobalValue := global.SecurityGroups
			global.TaskDefinition = tt.taskDefinitionOverrideValue
			global.PlatformVersion = tt.platformVersionOverrideValue
			global.Subnets = tt.subnetsOverrideValue
			global.SecurityGroups = tt.securityGroupsOverrideValue

			defer func() {
				global.TaskDefinition = oldTaskDefGlobalValue
				global.PlatformVersion = oldPlatformVersionGlobalValue
				global.Subnets = oldSubnetsGlobalValue
				global.SecurityGroups = oldSecurityGroupsGlobalValue
			}()

			testContext := createCliContextForTests(originalTaskDefinition, originalPlatformVersion, originalSubnets, originalSecurityGroups)
			assert.Equal(t, originalTaskDefinition, testContext.Config().Fargate.TaskDefinition)
			assert.Equal(t, originalPlatformVersion, testContext.Config().Fargate.PlatformVersion)
			assert.Equal(t, originalSubnets, testContext.Config().Fargate.Subnets)
			assert.Equal(t, originalSecurityGroups, testContext.Config().Fargate.SecurityGroups)

			err := loadCliArgsEnvVars(testContext)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedTaskDefinition, testContext.Config().Fargate.TaskDefinition)
			assert.Equal(t, tt.expectedPlatformVersion, testContext.Config().Fargate.PlatformVersion)
			assert.Equal(t, tt.expectedSubnets, testContext.Config().Fargate.Subnets)
			assert.Equal(t, tt.expectedSecurityGroups, testContext.Config().Fargate.SecurityGroups)
		})
	}
}

func createCliContextForTests(taskDefinition string, platformVersion string, subnets []string, securityGroups []string) *cli.Context {
	cliCtx := new(cli.Context)
	cliCtx.SetConfig(config.Global{
		Fargate: config.Fargate{
			TaskDefinition:  taskDefinition,
			PlatformVersion: platformVersion,
			Subnets:         subnets,
			SecurityGroups:  securityGroups,
		},
	})

	return cliCtx
}
