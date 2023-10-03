// Package task centralizes code related to managing tasks in the application
package task

import (
	"bytes"
	"fmt"
	"os"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/encoding"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/env"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
)

type (
	// MetadataManager represents a repository to store temporary data related to task information
	MetadataManager interface {
		// Persist stores the desired data
		Persist(data Data) error

		// Get fetches existing data from the metadata storage
		Get() (Data, error)

		// Clear removes existing data
		Clear() error
	}

	// Data centralizes attributes that should be persisted in the metadata storage
	Data struct {
		TaskARN     string `json:"task_arn,omitempty"`
		ContainerIP string `json:"container_ip,omitempty"`
		PrivateKey  []byte `json:"private_key,omitempty"`
	}

	JobResponse struct {
		Fargate Data `json:"fargate,omitempty"`
	}
)

// jobResponseMetadataManager is an implementation of MetadataManager that uses
// the job response file populated by runner at the beginning of a job. It
// persists the fargate specific data by loading and augmenting the job
// response file, then writing it back. The Get() implementation ignores all
// other data in the job response file.
type jobResponseMetadataManager struct {
	encoder             encoding.Encoder
	logger              logging.Logger
	jobResponseFilename string
}

func NewJobResponseMetadataManager(logger logging.Logger) MetadataManager {
	return &jobResponseMetadataManager{
		logger:              logger,
		jobResponseFilename: env.New().Get("JOB_RESPONSE_FILE"),
		encoder:             encoding.NewJSON(),
	}
}

func (m *jobResponseMetadataManager) Persist(data Data) error {
	m.logger.Debug("[Persist] Will persist metadata")
	return m.augment(&data)
}

func (m *jobResponseMetadataManager) Get() (Data, error) {
	m.logger.Debug("[Get] Will get existing metadata")
	jr := JobResponse{}
	err := m.load(&jr)
	return jr.Fargate, err
}

func (m *jobResponseMetadataManager) Clear() error {
	// this is a no-op because runner takes care of this...
	return nil
}

func (m *jobResponseMetadataManager) load(dest any) error {
	raw, err := os.ReadFile(m.jobResponseFilename)
	if err != nil {
		return fmt.Errorf("reading file %q: %w", m.jobResponseFilename, err)
	}

	err = m.encoder.Decode(bytes.NewBuffer(raw), dest)
	if err != nil {
		return fmt.Errorf("decoding JSON: %w", err)
	}

	return nil
}

func (m *jobResponseMetadataManager) save(data any) error {
	buf := new(bytes.Buffer)
	if err := m.encoder.Encode(data, buf); err != nil {
		return fmt.Errorf("encoding data to JSON: %w", err)
	}

	if err := os.WriteFile(m.jobResponseFilename, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("writing file %q: %w", m.jobResponseFilename, err)
	}
	return nil
}

// augment reads the jobresponse file, augments it with fargate task
// information, and rewrites it to the same file.
func (m *jobResponseMetadataManager) augment(fargateData *Data) error {
	allData := map[string]any{}
	if err := m.load(&allData); err != nil {
		return err
	}
	allData["fargate"] = fargateData

	return m.save(allData)
}
