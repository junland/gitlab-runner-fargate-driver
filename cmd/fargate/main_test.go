package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/cli"
)

func TestLoadCliArgsEnvVars(t *testing.T) {
	defaultTaskDefinition := "default-task-def:1"
	overrideTaskDefinition := "another-task-def:1"
	defaultPlatformVersion := "LATEST"
	overridePlatformVersion := "1.4.0"

	taskDefinitionTests := map[string]struct {
		taskDefinitionOverrideValue string
		expectedTaskDefinition      string
	}{
		"Should not override TaskDefinition if not received by command line or env variable": {
			taskDefinitionOverrideValue: "",
			expectedTaskDefinition:      defaultTaskDefinition,
		},
		"Should override TaskDefinition if received by command line or env variable": {
			taskDefinitionOverrideValue: overrideTaskDefinition,
			expectedTaskDefinition:      overrideTaskDefinition,
		},
	}

	for tn, tt := range taskDefinitionTests {
		t.Run(tn, func(t *testing.T) {
			originalGlobalValue := global.TaskDefinition
			defer func() {
				global.TaskDefinition = originalGlobalValue
			}()
			global.TaskDefinition = tt.taskDefinitionOverrideValue

			testContext := createCliContextForTaskDefinitionTests(defaultTaskDefinition)
			assert.Equal(t, defaultTaskDefinition, testContext.Config().Fargate.TaskDefinition)

			err := loadCliArgsEnvVars(testContext)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedTaskDefinition, testContext.Config().Fargate.TaskDefinition)
		})
	}

	platformVersionTests := map[string]struct {
		platformVersionOverrideValue string
		expectedPlatformVersion      string
	}{
		"Should not override PlatformVersion if not received by command line or env variable": {
			platformVersionOverrideValue: "",
			expectedPlatformVersion:      defaultPlatformVersion,
		},
		"Should override PlatformVersion if received by command line or env variable": {
			platformVersionOverrideValue: overridePlatformVersion,
			expectedPlatformVersion:      overridePlatformVersion,
		},
	}

	for tn, tt := range platformVersionTests {
		t.Run(tn, func(t *testing.T) {
			originalGlobalValue := global.PlatformVersion
			defer func() {
				global.PlatformVersion = originalGlobalValue
			}()
			global.PlatformVersion = tt.platformVersionOverrideValue

			testContext := createCliContextForPlatformVersionTests(defaultPlatformVersion)
			assert.Equal(t, defaultPlatformVersion, testContext.Config().Fargate.PlatformVersion)

			err := loadCliArgsEnvVars(testContext)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPlatformVersion, testContext.Config().Fargate.PlatformVersion)
		})
	}
}

func createCliContextForTaskDefinitionTests(taskDefinition string) *cli.Context {
	cliCtx := new(cli.Context)
	cliCtx.SetConfig(config.Global{
		Fargate: config.Fargate{TaskDefinition: taskDefinition},
	})

	return cliCtx
}

func createCliContextForPlatformVersionTests(platformVersion string) *cli.Context {
	cliCtx := new(cli.Context)

	cliCtx.SetConfig(config.Global{
		Fargate: config.Fargate{PlatformVersion: platformVersion},
	})
	return cliCtx
}
