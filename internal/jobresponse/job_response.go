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
		ShortToken string     `json:"token"`
		Variables  []variable `json:"variables,omitempty"`
		JobID      int64      `json:"id"`
	}
)

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
