package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateFilename(t *testing.T) {
	tests := map[string]struct {
		shortToken       string
		projectURL       string
		pipelineID       int64
		jobID            int64
		expectedFilename string
	}{
		"Valid parameters": {
			shortToken:       "test",
			projectURL:       "http://testing.com",
			pipelineID:       155678,
			jobID:            215487,
			expectedFilename: "f5b00218f7b028e8f84cb27f893d388ad81fdc66",
		},
		"Zeroed parameters": {
			shortToken:       "",
			projectURL:       "",
			pipelineID:       0,
			jobID:            0,
			expectedFilename: "12a519293788ee661cff04d149c3226a47096efd",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			runnerData := RunnerData{
				ShortToken: tt.shortToken,
				ProjectURL: tt.projectURL,
				PipelineID: tt.pipelineID,
				JobID:      tt.jobID,
			}
			filename := GenerateFilename(runnerData)

			assert.Equal(t, filename, tt.expectedFilename, "wrong identifier calculated")
		})
	}
}
