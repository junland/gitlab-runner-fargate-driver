package jobresponse

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadJobResponseFile(t *testing.T) {
	t.Run("file exists and is valid", func(t *testing.T) {
		os.Setenv(JobResponseFileVar, "testdata/response.json")
		defer os.Unsetenv(JobResponseFileVar)

		jrf := Load()

		assert.Equal(t, int64(123456789), jrf.JobID)
		assert.Equal(t, "sdlkfjg;pi[p2309ir[093irpx2utp2985", jrf.ShortToken)
		assert.Equal(t, int64(987654321), jrf.PipelineID())
		assert.Equal(t, "https://gitlab.com/blammo/some-awesome-project", jrf.ProjectURL())
	})

	t.Run("JOB_RESPONSE_FILE vr not set", func(t *testing.T) {
		require.Panics(t, func() {
			Load()
		})
	})

	t.Run("file does not exist", func(t *testing.T) {
		os.Setenv(JobResponseFileVar, "blammo")
		defer os.Unsetenv(JobResponseFileVar)

		require.Panics(t, func() {
			Load()
		})
	})
}

func TestJobResponse_panics(t *testing.T) {
	t.Run("missing CI_PIPELINE_ID variable", func(t *testing.T) {
		require.Panics(t, func() {
			jr := JobResponse{}

			jr.PipelineID()
		})
	})

	t.Run("empty required variable", func(t *testing.T) {
		require.Panics(t, func() {
			jr := JobResponse{
				Variables: []variable{
					{
						Key: "CI_PIPELINE_ID",
					},
				},
			}
			// could also be ProjectURL()
			jr.PipelineID()
		})
	})

	t.Run("CI_PIPELINE_ID not int64", func(t *testing.T) {
		require.Panics(t, func() {
			jr := JobResponse{
				Variables: []variable{
					{
						Key:   "CI_PIPELINE_ID",
						Value: "this is not an int64",
					},
				},
			}
			jr.PipelineID()
		})
	})

	t.Run("CI_PROJECT_URL missing", func(t *testing.T) {
		require.Panics(t, func() {
			jr := JobResponse{}
			jr.ProjectURL()
		})
	})
}
