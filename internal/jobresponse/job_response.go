package jobresponse

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/encoding"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/env"
)

const JobResponseFileVar = "JOB_RESPONSE_FILE"

type (
	variable struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	JobResponse struct {
		Token     string     `json:"token"`
		Variables []variable `json:"variables,omitempty"`
		JobID     int64      `json:"id"`
	}
)

func (j *JobResponse) ShortToken() string {
	return shortenToken(j.Token)
}

func (j *JobResponse) PipelineID() int64 {
	return j.getInt64Variable("CI_PIPELINE_ID")
}

func (j *JobResponse) ProjectURL() string {
	return j.getVariable("CI_PROJECT_URL")
}

func (j *JobResponse) getVariable(key string) string {
	for _, v := range j.Variables {
		if v.Key == key {
			if strings.TrimSpace(v.Value) == "" {
				panic(fmt.Sprintf("no value for variable %q in job response file", v.Key))
			}
			return v.Value
		}
	}
	panic(fmt.Sprintf("variable %q not defined in job response file", key))
}

func (j *JobResponse) getInt64Variable(key string) int64 {
	val, err := strconv.ParseInt(j.getVariable(key), 10, 64)
	if err != nil {
		panic(fmt.Errorf("%q variable is not an int64: %w", key, err))
	}
	return val
}

func Load() JobResponse {
	jobResponseFilename := strings.TrimSpace(env.New().Get(JobResponseFileVar))
	if jobResponseFilename == "" {
		panic(fmt.Errorf("%s variable not defined", JobResponseFileVar))
	}

	raw, err := os.ReadFile(jobResponseFilename)
	if err != nil {
		panic(fmt.Errorf("reading file %q: %w", jobResponseFilename, err))
	}

	jr := JobResponse{}
	encoder := encoding.NewJSON()
	err = encoder.Decode(bytes.NewBuffer(raw), &jr)
	if err != nil {
		panic(fmt.Errorf("decoding JSON: %w", err))
	}

	return jr
}

// Copied from https://gitlab.com/gitlab-org/gitlab-runner/-/blob/main/helpers/shorten_token.go
func shortenToken(token string) string {
	switch {
	case len(token) < 8:
		return token
	case token[:5] == "glrt-":
		// Token is prefixed with CREATED_RUNNER_TOKEN_PREFIX: glrt- (for Gitlab Runner Token).
		// Let's add some more characters in order to compensate.
		return token[5:14]
	case token[:2] == "GR" && len(token) >= 17 && strings.IndexFunc(token[2:9], isInvalidPrefixRune) == -1:
		// Token is prefixed with RUNNERS_TOKEN_PREFIX: GR (for Gitlab Runner) combined with the rotation
		// date decimal-to-hex-encoded. Let's add some more characters in order to compensate.
		return token[:17]
	default:
		return token[:8]
	}
}

func isInvalidPrefixRune(r rune) bool {
	return (r < '0' || r > '9') && (r < 'A' || r > 'F') && (r < 'a' || r > 'f')
}
