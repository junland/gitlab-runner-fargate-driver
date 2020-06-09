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
	overrideValueForTaskDefinition := "another-task-def:1"
	overrideValueForPlatformVersion := "1.4.0"

	tests := map[string]struct {
		taskDefinitionOverrideValue  string
		platformVersionOverrideValue string
		expectedTaskDefinition       string
		expectedPlatformVersion      string
	}{
		"Should keep original values if nothing received by command line or env variable": {
			taskDefinitionOverrideValue: "",
			expectedTaskDefinition:      originalTaskDefinition,
			expectedPlatformVersion:     originalPlatformVersion,
		},
		"Should override task definition if received by command line or env variable": {
			taskDefinitionOverrideValue: overrideValueForTaskDefinition,
			expectedTaskDefinition:      overrideValueForTaskDefinition,
			expectedPlatformVersion:     originalPlatformVersion,
		},
		"Should override platform version if received by command line or env variable": {
			platformVersionOverrideValue: overrideValueForPlatformVersion,
			expectedTaskDefinition:       originalTaskDefinition,
			expectedPlatformVersion:      overrideValueForPlatformVersion,
		},
		"Should override platform version and task definition if received by command line or env variable": {
			taskDefinitionOverrideValue:  overrideValueForTaskDefinition,
			platformVersionOverrideValue: overrideValueForPlatformVersion,
			expectedTaskDefinition:       overrideValueForTaskDefinition,
			expectedPlatformVersion:      overrideValueForPlatformVersion,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			originalTaskDefGlobalValue := global.TaskDefinition
			originalPlatVersionGlobalValue := global.PlatformVersion
			global.TaskDefinition = tt.taskDefinitionOverrideValue
			global.PlatformVersion = tt.platformVersionOverrideValue

			defer func() {
				global.TaskDefinition = originalTaskDefGlobalValue
				global.PlatformVersion = originalPlatVersionGlobalValue
			}()

			testContext := createCliContextForTests(originalTaskDefinition, originalPlatformVersion)
			assert.Equal(t, originalTaskDefinition, testContext.Config().Fargate.TaskDefinition)
			assert.Equal(t, originalPlatformVersion, testContext.Config().Fargate.PlatformVersion)

			err := loadCliArgsEnvVars(testContext)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedTaskDefinition, testContext.Config().Fargate.TaskDefinition)
			assert.Equal(t, tt.expectedPlatformVersion, testContext.Config().Fargate.PlatformVersion)
		})
	}
}

func createCliContextForTests(taskDefinition string, platformVersion string) *cli.Context {
	cliCtx := new(cli.Context)
	cliCtx.SetConfig(config.Global{
		Fargate: config.Fargate{
			TaskDefinition:  taskDefinition,
			PlatformVersion: platformVersion,
		},
	})

	return cliCtx
}
