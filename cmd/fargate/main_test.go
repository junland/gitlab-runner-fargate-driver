package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/cli"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
)

func TestLoadCliArgsEnvVars(t *testing.T) {
	originalTaskDefinition := "default-task-def:1"
	originalPlatformVersion := "LATEST"
	originalSubnet := "subnet-1"
	originalSecurityGroup := "sg-123456"
	originalRoleARN := "arn:aws:iam::xxxxxxxxx:role/role-name"
	originalSessionVars := "CI_PROJECT_ID"
	originalEnableExecuteCommand := false
	overrideTaskDefinition := "another-task-def:1"
	overridePlatformVersion := "1.4.0"
	overrideSubnets := "subnet-1"         //,subnet-2"
	overrideSecurityGroups := "sg-123456" //,sg-654321"
	expectedOverrideSubnets := overrideSubnets
	expectedOverrideSecurityGroups := overrideSecurityGroups
	overrideRoleARN := "arn:aws:iam::xxxxxxxxx:role/another-role-name"
	overrideSessionVars := "CI_PROJECT_ID,CI_PROJECT_PATH"
	overrideEnableExecuteCommand := true
	expectedOverrideExecutecommand := overrideEnableExecuteCommand

	tests := map[string]struct {
		taskDefinitionOverrideValue  string
		platformVersionOverrideValue string
		subnetOverrideValue          string
		securityGroupOverrideValue   string
		roleARNOverrideValue         string
		sessionVarsOverrideValue     string
		executeCommandOverrideValue  *bool
		expectedTaskDefinition       string
		expectedPlatformVersion      string
		expectedSubnet               string
		expectedSecurityGroup        string
		expectedRoleARN              string
		expectedSessionVars          string
		expectedEnableExecuteCommand bool
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
			expectedRoleARN:              originalRoleARN,
			expectedSessionVars:          originalSessionVars,
			expectedEnableExecuteCommand: originalEnableExecuteCommand,
		},
		"Should override task definition if received by command line or env variable": {
			taskDefinitionOverrideValue: overrideTaskDefinition,
			expectedTaskDefinition:      overrideTaskDefinition,
			expectedPlatformVersion:     originalPlatformVersion,
			expectedSubnet:              originalSubnet,
			expectedSecurityGroup:       originalSecurityGroup,
			expectedRoleARN:             originalRoleARN,
			expectedSessionVars:         originalSessionVars,
		},
		"Should override platform version if received by command line or env variable": {
			platformVersionOverrideValue: overridePlatformVersion,
			expectedTaskDefinition:       originalTaskDefinition,
			expectedPlatformVersion:      overridePlatformVersion,
			expectedSubnet:               originalSubnet,
			expectedSecurityGroup:        originalSecurityGroup,
			expectedRoleARN:              originalRoleARN,
			expectedSessionVars:          originalSessionVars,
		},
		"Should override subnets if received by command line or env variable": {
			subnetOverrideValue:     overrideSubnets,
			expectedSubnet:          expectedOverrideSubnets,
			expectedTaskDefinition:  originalTaskDefinition,
			expectedPlatformVersion: originalPlatformVersion,
			expectedSecurityGroup:   originalSecurityGroup,
			expectedRoleARN:         originalRoleARN,
			expectedSessionVars:     originalSessionVars,
		},
		"Should override security groups if received by command line or env variable": {
			securityGroupOverrideValue: overrideSecurityGroups,
			expectedSecurityGroup:      expectedOverrideSecurityGroups,
			expectedTaskDefinition:     originalTaskDefinition,
			expectedPlatformVersion:    originalPlatformVersion,
			expectedSubnet:             originalSubnet,
			expectedRoleARN:            originalRoleARN,
			expectedSessionVars:        originalSessionVars,
		},
		"Should override platform version and task definition if received by command line or env variable": {
			taskDefinitionOverrideValue:  overrideTaskDefinition,
			platformVersionOverrideValue: overridePlatformVersion,
			expectedTaskDefinition:       overrideTaskDefinition,
			expectedPlatformVersion:      overridePlatformVersion,
			expectedSubnet:               originalSubnet,
			expectedSecurityGroup:        originalSecurityGroup,
			expectedRoleARN:              originalRoleARN,
			expectedSessionVars:          originalSessionVars,
		},
		"Should override subnets and security groups if received by command line or env variable": {
			subnetOverrideValue:        overrideSubnets,
			securityGroupOverrideValue: overrideSecurityGroups,
			expectedSubnet:             expectedOverrideSubnets,
			expectedSecurityGroup:      expectedOverrideSecurityGroups,
			expectedTaskDefinition:     originalTaskDefinition,
			expectedPlatformVersion:    originalPlatformVersion,
			expectedRoleARN:            originalRoleARN,
			expectedSessionVars:        originalSessionVars,
		},
		"Should override all possible params if received by command line or env variable": {
			taskDefinitionOverrideValue:  overrideTaskDefinition,
			platformVersionOverrideValue: overridePlatformVersion,
			subnetOverrideValue:          overrideSubnets,
			securityGroupOverrideValue:   overrideSecurityGroups,
			executeCommandOverrideValue:  &overrideEnableExecuteCommand,
			expectedSubnet:               expectedOverrideSubnets,
			expectedSecurityGroup:        expectedOverrideSecurityGroups,
			expectedTaskDefinition:       overrideTaskDefinition,
			expectedPlatformVersion:      overridePlatformVersion,
			expectedRoleARN:              originalRoleARN,
			expectedSessionVars:          originalSessionVars,
			expectedEnableExecuteCommand: expectedOverrideExecutecommand,
		},
		"Should override role arn if received by command line or env variable": {
			roleARNOverrideValue:    overrideRoleARN,
			expectedSubnet:          expectedOverrideSubnets,
			expectedSecurityGroup:   expectedOverrideSecurityGroups,
			expectedTaskDefinition:  originalTaskDefinition,
			expectedPlatformVersion: originalPlatformVersion,
			expectedRoleARN:         overrideRoleARN,
			expectedSessionVars:     originalSessionVars,
		},
		"Should override role arn and variables if received by command line or env variable": {
			roleARNOverrideValue:     overrideRoleARN,
			sessionVarsOverrideValue: overrideSessionVars,
			expectedSubnet:           expectedOverrideSubnets,
			expectedSecurityGroup:    expectedOverrideSecurityGroups,
			expectedTaskDefinition:   originalTaskDefinition,
			expectedPlatformVersion:  originalPlatformVersion,
			expectedRoleARN:          overrideRoleARN,
			expectedSessionVars:      overrideSessionVars,
		},
		"Should override enable execute command if received by command line or env variable": {
			executeCommandOverrideValue:  &overrideEnableExecuteCommand,
			expectedSubnet:               expectedOverrideSubnets,
			expectedSecurityGroup:        expectedOverrideSecurityGroups,
			expectedTaskDefinition:       originalTaskDefinition,
			expectedPlatformVersion:      originalPlatformVersion,
			expectedRoleARN:              originalRoleARN,
			expectedSessionVars:          originalSessionVars,
			expectedEnableExecuteCommand: overrideEnableExecuteCommand,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			oldTaskDefGlobalValue := global.TaskDefinition
			oldPlatformVersionGlobalValue := global.PlatformVersion
			oldSubnetGlobalValue := global.Subnet
			oldSecurityGroupGlobalValue := global.SecurityGroup
			oldRoleARNGlobalValue := global.RoleARN
			oldVariablesGlobalValue := global.SessionVars
			oldEnableExecuteCommandGlobalValue := global.EnableExecuteCommand
			global.TaskDefinition = tt.taskDefinitionOverrideValue
			global.PlatformVersion = tt.platformVersionOverrideValue
			global.Subnet = tt.subnetOverrideValue
			global.SecurityGroup = tt.securityGroupOverrideValue
			global.RoleARN = tt.roleARNOverrideValue
			global.SessionVars = tt.sessionVarsOverrideValue
			global.EnableExecuteCommand = tt.executeCommandOverrideValue

			defer func() {
				global.TaskDefinition = oldTaskDefGlobalValue
				global.PlatformVersion = oldPlatformVersionGlobalValue
				global.Subnet = oldSubnetGlobalValue
				global.SecurityGroup = oldSecurityGroupGlobalValue
				global.RoleARN = oldRoleARNGlobalValue
				global.SessionVars = oldVariablesGlobalValue
				global.EnableExecuteCommand = oldEnableExecuteCommandGlobalValue
			}()

			testContext := createCliContextForTests(originalTaskDefinition, originalPlatformVersion, originalSubnet,
				originalSecurityGroup, originalRoleARN, originalSessionVars)
			assert.Equal(t, originalTaskDefinition, testContext.Config().Fargate.TaskDefinition)
			assert.Equal(t, originalPlatformVersion, testContext.Config().Fargate.PlatformVersion)
			assert.Equal(t, originalSubnet, testContext.Config().Fargate.Subnet)
			assert.Equal(t, originalSecurityGroup, testContext.Config().Fargate.SecurityGroup)
			assert.Equal(t, originalRoleARN, testContext.Config().STSConfiguration.RoleARN)
			assert.Equal(t, originalSessionVars, testContext.Config().STSConfiguration.SessionVars)
			assert.Equal(t, originalEnableExecuteCommand, testContext.Config().Fargate.EnableExecuteCommand)

			err := loadCliArgsEnvVars(testContext)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedTaskDefinition, testContext.Config().Fargate.TaskDefinition)
			assert.Equal(t, tt.expectedPlatformVersion, testContext.Config().Fargate.PlatformVersion)
			assert.Equal(t, tt.expectedSubnet, testContext.Config().Fargate.Subnet)
			assert.Equal(t, tt.expectedSecurityGroup, testContext.Config().Fargate.SecurityGroup)
			assert.Equal(t, tt.expectedRoleARN, testContext.Config().STSConfiguration.RoleARN)
			assert.Equal(t, tt.expectedSessionVars, testContext.Config().STSConfiguration.SessionVars)
			assert.Equal(t, tt.expectedEnableExecuteCommand, testContext.Config().Fargate.EnableExecuteCommand)
		})
	}
}

func TestAssertRequiredConfig(t *testing.T) {
	originalTaskDefinition := "default-task-def:1"
	originalPlatformVersion := "LATEST"
	originalSubnet := "subnet-1"
	originalSecurityGroup := "sg-123456"
	originalCluster := "my-cluster"

	cliCtx := new(cli.Context)
	cliCtx.SetLogger(test.NewNullLogger())
	cliCtx.SetConfig(config.Global{
		Fargate: config.Fargate{
			Cluster:         originalCluster,
			Region:          "eu-west-1",
			PlatformVersion: originalPlatformVersion,
			SecurityGroup:   originalSecurityGroup,
			Subnet:          originalSubnet,
			TaskDefinition:  originalTaskDefinition,
		},
	})

	err := assertRequiredConfig(cliCtx)
	assert.NoError(t, err)
}

func TestAssertRequiredConfigStoresTheDefaultedWaitSettings(t *testing.T) {
	cliCtx := new(cli.Context)
	cliCtx.SetLogger(test.NewNullLogger())
	cliCtx.SetConfig(config.Global{
		Fargate: config.Fargate{
			Cluster:              "my-cluster",
			Region:               "eu-west-1",
			SecurityGroup:        "sg-123456",
			Subnet:               "subnet-1",
			TaskDefinition:       "default-task-def:1",
			TaskWaitConfig:       config.TaskWaitConfig{MaxAttempts: 0, WaitTimeout: 7},
			TaskHealthWaitConfig: config.TaskHealthWaitConfig{Retries: 5, CheckInterval: 0},
		},
	})

	err := assertRequiredConfig(cliCtx)

	assert.NoError(t, err)
	assert.Equal(t,
		config.TaskWaitConfig{MaxAttempts: config.AWSWaitDefaultMaxAttempts, WaitTimeout: 7},
		cliCtx.Config().Fargate.TaskWaitConfig)
	assert.Equal(t,
		config.TaskHealthWaitConfig{Retries: 5, CheckInterval: config.AWSWaitTaskHealthDefaultCheckInterval},
		cliCtx.Config().Fargate.TaskHealthWaitConfig)
}

func createCliContextForTests(taskDefinition, platformVersion, subnet, securityGroup, roleARN, sessionVars string) *cli.Context {
	cliCtx := new(cli.Context)
	cliCtx.SetConfig(config.Global{
		Fargate: config.Fargate{
			TaskDefinition:  taskDefinition,
			PlatformVersion: platformVersion,
			Subnet:          subnet,
			SecurityGroup:   securityGroup,
		},
		STSConfiguration: config.STSConfiguration{
			RoleARN:     roleARN,
			SessionVars: sessionVars,
		},
	})

	return cliCtx
}
