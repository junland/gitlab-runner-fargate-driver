// Package task centralizes code related to managing tasks in the application
package task

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/encoding"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/fs"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/runner"
)

// ErrFileNotExists is returned when trying to operate in a file that does not exist
var ErrFileNotExists = errors.New("file does not exist")

// MetadataManager represents a repository to store temporary data related to task information
type MetadataManager interface {
	// Persist stores the desired data
	Persist(fargateTask FargateTaskData) error

	// Get fetches existing data from the metadata storage
	Get() (FargateTaskData, error)

	// Clear removes existing data
	Clear() error
}

// FargateTaskData centralizes data that should be persisted in the metadata storage
type FargateTaskData struct {
	TaskARN     string
	ContainerIP string
}

type fsMetadataManager struct {
	logger    logging.Logger
	fs        fs.FS
	encoder   encoding.Encoder
	directory string
	filename  string
}

// NewMetadataManager is a constructor for the concrete type of the MetadataManager interface
func NewMetadataManager(logger logging.Logger, filesDir string) MetadataManager {
	manager := new(fsMetadataManager)

	manager.logger = logger
	manager.fs = fs.NewOS()
	manager.encoder = encoding.NewJSON()
	manager.directory = filesDir

	runnerData := RunnerData{
		ShortToken: runner.GetAdapter().ShortToken(),
		ProjectURL: runner.GetAdapter().ProjectURL(),
		PipelineID: runner.GetAdapter().PipelineID(),
		JobID:      runner.GetAdapter().JobID(),
	}
	manager.filename = GenerateFilename(runnerData)

	return manager
}

func (f *fsMetadataManager) Persist(fargateTask FargateTaskData) error {
	f.logger.Debug("Will persist metadata")

	buf := new(bytes.Buffer)
	err := f.encoder.Encode(fargateTask, buf)
	if err != nil {
		return fmt.Errorf("error marshalling data: %w", err)
	}

	err = f.fs.WriteFile(f.getFullFilePath(), buf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	return nil
}

func (f *fsMetadataManager) getFullFilePath() string {
	return filepath.Join(f.directory, fmt.Sprintf("%s.json", f.filename))
}

func (f *fsMetadataManager) Get() (FargateTaskData, error) {
	f.logger.Debug("Will get existing metadata")

	fargateTaskData := FargateTaskData{}

	err := f.errIfFileNotExists()
	if err != nil {
		return fargateTaskData, fmt.Errorf("could not access file: %w", err)
	}

	data, err := f.fs.ReadFile(f.getFullFilePath())
	if err != nil {
		return fargateTaskData, fmt.Errorf("error reading file: %w", err)
	}

	err = f.encoder.Decode(bytes.NewBuffer(data), &fargateTaskData)
	if err != nil {
		return fargateTaskData, fmt.Errorf("error unmarshalling data: %w", err)
	}

	return fargateTaskData, nil
}

func (f *fsMetadataManager) errIfFileNotExists() error {
	exists, err := f.fs.Exists(f.getFullFilePath())
	if err != nil {
		return fmt.Errorf("error trying to access file: %w", err)
	}

	if !exists {
		return ErrFileNotExists
	}

	return nil
}

func (f *fsMetadataManager) Clear() error {
	f.logger.Debug("Will delete existing metadata")

	err := f.errIfFileNotExists()
	if err != nil {
		return fmt.Errorf("could not access file: %w", err)
	}

	err = f.fs.Remove(f.getFullFilePath())
	if err != nil {
		return fmt.Errorf("error deleting file: %w", err)
	}

	return nil
}
