package task

import (
	"crypto/sha256"
	"fmt"
)

// RunnerData centralizes the necessary attributes for generating the filename
type RunnerData struct {
	ShortToken string
	ProjectURL string
	PipelineID int64
	JobID      int64
}

// GenerateFilename returns a statically generated filename considering some environment parameters
func GenerateFilename(runnerData RunnerData) string {
	longID := fmt.Sprintf(
		"%s-%s-%d-%d",
		runnerData.ShortToken,
		runnerData.ProjectURL,
		runnerData.PipelineID,
		runnerData.JobID,
	)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(longID)))

	return hash[:40]
}
