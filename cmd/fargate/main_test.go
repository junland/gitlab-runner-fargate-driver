package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/config"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/cli"
)

func TestLoadCliArgsEnvVars(t *testing.T) {
	defaultTaskDefinition := "default-task-def:1"
	overrideTaskDefinition := "another-task-def:1"

	tests := map[string]struct {
		overrideValue          string
		expectedTaskDefinition string
	}{
		"Should not override if not received by command line or env variable": {
			overrideValue:          "",
			expectedTaskDefinition: defaultTaskDefinition,
		},
		"Should override if received by command line or env variable": {
			overrideValue:          overrideTaskDefinition,
			expectedTaskDefinition: overrideTaskDefinition,
		},
	}

	for tn, tt := range tests {
		originalGlobalValue := global.TaskDefinition
		global.TaskDefinition = tt.overrideValue
		defer func(originalValue string) {
			global.TaskDefinition = originalValue
		}(originalGlobalValue)

		t.Run(tn, func(t *testing.T) {
			testContext := createCliContextForTests(defaultTaskDefinition)

			err := loadCliArgsEnvVars(testContext)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedTaskDefinition, testContext.Config().Fargate.TaskDefinition)
		})
	}
}

func createCliContextForTests(taskDefinition string) *cli.Context {
	cliCtx := new(cli.Context)
	cliCtx.SetConfig(config.Global{
		Fargate: config.Fargate{TaskDefinition: taskDefinition},
	})
	return cliCtx
}
