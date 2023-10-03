// Package task centralizes code related to managing tasks in the application
package task

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/encoding"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/env"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/fs"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/runner"
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

	fsMetadataManager struct {
		logger    logging.Logger
		fs        fs.FS
		encoder   encoding.Encoder
		directory string
		filename  string
	}

	JobResponse struct {
		Fargate Data `json:"fargate,omitempty"`
	}
)

// NewMetadataManager is a constructor for the concrete type of the MetadataManager interface
func NewMetadataManager(logger logging.Logger, filesDir string) MetadataManager {
	manager := new(fsMetadataManager)

	manager.logger = logger
	manager.fs = fs.NewOS()
	manager.encoder = encoding.NewJSON()
	manager.directory = filesDir

	runnerAdapter := runner.GetAdapter()
	runnerData := RunnerData{
		ShortToken: runnerAdapter.ShortToken(),
		ProjectURL: runnerAdapter.ProjectURL(),
		PipelineID: runnerAdapter.PipelineID(),
		JobID:      runnerAdapter.JobID(),
	}
	manager.filename = GenerateFilename(runnerData)

	return manager
}

// metadata persist impl...
func (f *fsMetadataManager) Persist(data Data) error {
	f.logger.Debug("[Persist] Will persist metadata")

	buf := new(bytes.Buffer)
	err := f.encoder.Encode(data, buf)
	if err != nil {
		return fmt.Errorf("encoding data to JSON: %w", err)
	}

	err = f.fs.WriteFile(f.getFullFilePath(), buf.Bytes(), 0o600)
	if err != nil {
		return fmt.Errorf("writing file %q: %w", f.getFullFilePath(), err)
	}

	f.logger.Debug("[Persist] Metadata was persisted")

	return nil
}

func (f *fsMetadataManager) getFullFilePath() string {
	return filepath.Join(f.directory, fmt.Sprintf("%s.json", f.filename))
}

// metadata read impl...
func (f *fsMetadataManager) Get() (Data, error) {
	f.logger.Debug("[Get] Will get existing metadata")

	data := Data{}

	err := f.errIfFileNotExists()
	if err != nil {
		return data, fmt.Errorf("trying to access file %q: %w", f.getFullFilePath(), err)
	}

	fileContent, err := f.fs.ReadFile(f.getFullFilePath())
	if err != nil {
		return data, fmt.Errorf("reading file %q: %w", f.getFullFilePath(), err)
	}

	err = f.encoder.Decode(bytes.NewBuffer(fileContent), &data)
	if err != nil {
		return data, fmt.Errorf("decoding JSON: %w", err)
	}

	f.logger.Debug("[Get] Metadata was fetched")

	return data, nil
}

func (f *fsMetadataManager) errIfFileNotExists() error {
	exists, err := f.fs.Exists(f.getFullFilePath())
	if err != nil {
		return err
	}

	if !exists {
		return os.ErrNotExist
	}

	return nil
}

func (f *fsMetadataManager) Clear() error {
	f.logger.Debug("[Clear] Will delete existing metadata")

	err := f.errIfFileNotExists()
	if err != nil {
		return fmt.Errorf("trying to access file %q: %w", f.getFullFilePath(), err)
	}

	err = f.fs.Remove(f.getFullFilePath())
	if err != nil {
		return fmt.Errorf("deleting file %q: %w", f.getFullFilePath(), err)
	}

	f.logger.Debug("[Clear] Metadata was deleted")

	return nil
}

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
