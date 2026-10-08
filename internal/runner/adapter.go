package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/fs"
	"gitlab.com/gitlab-org/gitlab-runner/executors/custom/api"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/encoding"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/env"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
)

const (
	runnerShortTokenVariable = "CUSTOM_ENV_CI_RUNNER_SHORT_TOKEN"
	runnerProjectURLVariable = "CUSTOM_ENV_CI_PROJECT_URL"
	runnerPipelineIDVariable = "CUSTOM_ENV_CI_PIPELINE_ID"
	runnerJobIDVariable      = "CUSTOM_ENV_CI_JOB_ID"

	// Mirrors the custom executor contract introduced in GitLab Runner 17.9.0 (MR !5028).
	runnerBuildExitCodeFileVariable = "BUILD_EXIT_CODE_FILE"

	unknownValue = "unknown"
)

var (
	osExiter    = os.Exit
	envResolver = env.New()
	fsResolver  = fs.NewOS()

	adapter *Adapter
)

type Adapter struct {
	buildFailureExitCode  int
	systemFailureExitCode int

	buildExitCodeFile string

	shortToken string
	projectURL string

	pipelineID int64
	jobID      int64
}

func (a *Adapter) GenerateExitFromError(err error, logger logging.Logger) {

	if !errors.Is(err, &BuildFailureError{}) {
		osExiter(a.systemFailureExitCode)
		return
	}

	var scriptExitCodeError *ScriptExitCodeError
	if errors.As(err, &scriptExitCodeError) && a.buildExitCodeFile != "" {
		exitCode := scriptExitCodeError.ExitStatus()
		a.writeBuildExitCodeFile(exitCode, logger)
	} else {
		a.removeBuildExitCodeFile(logger)
	}

	osExiter(a.buildFailureExitCode)
}

func (a *Adapter) writeBuildExitCodeFile(exitCode int, logger logging.Logger) {
	content := fmt.Appendf(nil, "%d\n", exitCode)

	if err := fsResolver.WriteFile(a.buildExitCodeFile, content, 0o644); err != nil {
		logger.WithError(err).Warn("Failed to write build exit code file")
	}
}

func (a *Adapter) removeBuildExitCodeFile(logger logging.Logger) {
	err := fsResolver.Remove(a.buildExitCodeFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.WithError(err).Warn("Failed to remove build exit code file")
	}
}

func (a *Adapter) ShortToken() string {
	return a.shortToken
}

func (a *Adapter) ProjectURL() string {
	return a.projectURL
}

func (a *Adapter) PipelineID() int64 {
	return a.pipelineID
}

func (a *Adapter) JobID() int64 {
	return a.jobID
}

func (a *Adapter) WriteCustomExecutorConfig(out io.Writer, hostname string) error {
	version := fargate.Version().ShortLine()
	cOut := api.ConfigExecOutput{
		Driver: &api.DriverInfo{
			Name:    &fargate.Version().Name,
			Version: &version,
		},
		Hostname: &hostname,
	}

	return encoding.NewJSON().Encode(cOut, out)
}

func InitAdapter() error {
	var err error

	adapter = new(Adapter)

	adapter.buildFailureExitCode, err = getExitCodeFromVariable(api.BuildFailureExitCodeVariable)
	if err != nil {
		return err
	}

	adapter.systemFailureExitCode, err = getExitCodeFromVariable(api.SystemFailureExitCodeVariable)
	if err != nil {
		return err
	}

	adapter.buildExitCodeFile = envResolver.Get(runnerBuildExitCodeFileVariable)

	adapter.shortToken = getVariableValueOrUnknown(runnerShortTokenVariable)
	adapter.projectURL = getVariableValueOrUnknown(runnerProjectURLVariable)

	adapter.pipelineID, err = getVariableInt64Value(runnerPipelineIDVariable)
	if err != nil {
		return err
	}

	adapter.jobID, err = getVariableInt64Value(runnerJobIDVariable)
	if err != nil {
		return err
	}

	return nil
}

func getExitCodeFromVariable(variable string) (int, error) {
	value := envResolver.Get(variable)
	exitCode, err := strconv.Atoi(value)
	if err != nil {
		return -1, fmt.Errorf("couldn't parse exit code %q from variable %q: %w", value, variable, err)
	}

	return exitCode, nil
}

func getVariableValueOrUnknown(variable string) string {
	value := envResolver.Get(variable)
	if value != "" {
		return value
	}

	return unknownValue
}

func getVariableInt64Value(variable string) (int64, error) {
	value := envResolver.Get(variable)
	if value == "" {
		return 0, nil
	}

	intValue, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("couldn't parse int64 value %q from variable %q: %w", value, variable, err)
	}

	return intValue, nil
}

func GetAdapter() *Adapter {
	if adapter == nil {
		panic("Runner Adapter not initialized. Must call runner.InitAdapter() first!")
	}
	return adapter
}
