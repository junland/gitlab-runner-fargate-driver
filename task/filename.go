package task

import (
	"crypto/sha256"
	"fmt"
)

type runnerData struct {
	ShortToken string
	ProjectURL string
	PipelineID int64
	JobID      int64
}

func generateFilename(data runnerData) string {
	longID := fmt.Sprintf(
		"%s-%d-%d",
		data.ProjectURL,
		data.PipelineID,
		data.JobID,
	)
	return fmt.Sprintf("%s-%x", data.ShortToken, sha256.Sum256([]byte(longID)))
}
