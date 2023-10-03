package task

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
)

func createTestLogger() logging.Logger {
	return test.NewNullLogger()
}

var (
	origJobResponseData = map[string]any{
		"this": 3.415,
		"is":   "blablaba",
		"some": map[string]any{
			"it":      "really",
			"doesn't": "matter",
			"what":    "this",
			"is":      "ok",
		},
		"data": nil,
	}
	fargateData = Data{
		TaskARN:     "taskarn",
		ContainerIP: "containerip",
		PrivateKey:  []byte("privatekey"),
	}
)

const jobResponseFileName = "jobresponse.json"

func TestJobResponseFileMetadataManager(t *testing.T) {
	defer os.Remove(jobResponseFileName)
	os.Setenv("JOB_RESPONSE_FILE", jobResponseFileName)

	// write initial job response file. IRL this part is done by runner.
	raw, err := json.Marshal(origJobResponseData)
	require.NoError(t, err)
	err = os.WriteFile(jobResponseFileName, raw, 0o600)
	require.NoError(t, err)

	md := NewJobResponseMetadataManager(createTestLogger())

	// load the original file, faraget task data should be empty.
	data, err := md.Get()
	require.NoError(t, err)
	assert.Equal(t, Data{}, data)

	// augment the existing data with fargate task data.
	err = md.Persist(fargateData)
	require.NoError(t, err)

	// reload the file, faraget task data should be populated.
	data, err = md.Get()
	require.NoError(t, err)
	assert.Equal(t, fargateData, data)

	// ensure none of the original data was modified or deleted.
	jrfmm, ok := md.(*jobResponseMetadataManager)
	require.True(t, ok)
	allData := map[string]any{}
	err = jrfmm.load(&allData)
	require.NoError(t, err)
	delete(allData, "fargate")
	assert.Equal(t, origJobResponseData, allData)
}
