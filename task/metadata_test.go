package task

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/assertions"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/encoding"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/fs"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/jobresponse"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/runner"
)

func TestNewMetadataManager(t *testing.T) {
	initializeAdapterForTesting(t)
	clear := initializeJobResponseFileForTesting(t)
	defer clear()

	manager := NewMetadataManager(createTestLogger(), "/tmp/")
	assert.NotNil(t, manager, "instance should have been created")
}

func initializeAdapterForTesting(t *testing.T) {
	err := os.Setenv("BUILD_FAILURE_EXIT_CODE", "1")
	require.NoError(t, err)

	err = os.Setenv("SYSTEM_FAILURE_EXIT_CODE", "1")
	require.NoError(t, err)

	err = runner.InitAdapter()
	require.NoError(t, err)
}

func initializeJobResponseFileForTesting(t *testing.T) func() {
	require.NoError(t, os.Setenv(jobresponse.JobResponseFileVar, "../internal/jobresponse/testdata/response.json"))
	return func() {
		require.NoError(t, os.Unsetenv(jobresponse.JobResponseFileVar))
	}
}

func createTestLogger() logging.Logger {
	return test.NewNullLogger()
}

func TestPersist(t *testing.T) {
	initializeAdapterForTesting(t)
	clear := initializeJobResponseFileForTesting(t)
	defer clear()

	testData := Data{
		TaskARN:     "task-arn",
		ContainerIP: "192.168.0.1",
		PrivateKey:  []byte("Testing"),
	}

	testDataWithoutPrivateKey := Data{
		TaskARN:     "task-arn",
		ContainerIP: "192.168.0.1",
	}

	testError := errors.New("simulated error")

	tests := map[string]struct {
		data           Data
		encodingError  error
		writeFileError error
		expectedError  error
	}{
		"Persist with success": {
			data:           testData,
			encodingError:  nil,
			writeFileError: nil,
			expectedError:  nil,
		},
		"Persist with success when no private key (backward compatibility)": {
			data:           testDataWithoutPrivateKey,
			encodingError:  nil,
			writeFileError: nil,
			expectedError:  nil,
		},
		"Error on encoding JSON": {
			data:           testData,
			encodingError:  testError,
			writeFileError: nil,
			expectedError:  testError,
		},
		"Error on persisting file": {
			data:           testData,
			encodingError:  nil,
			writeFileError: testError,
			expectedError:  testError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			mockFS := new(fs.MockFS)
			defer mockFS.AssertExpectations(t)

			mockEncoder := new(encoding.MockEncoder)
			defer mockEncoder.AssertExpectations(t)

			mockEncoder.On("Encode", tt.data, mock.AnythingOfType("*bytes.Buffer")).
				Return(tt.encodingError).
				Once()

			if tt.encodingError == nil {
				mockFS.On("WriteFile",
					mock.AnythingOfType("string"),
					mock.AnythingOfType("[]uint8"),
					mock.AnythingOfType("fs.FileMode"),
				).
					Return(tt.writeFileError).
					Once()
			}

			manager := NewMetadataManager(createTestLogger(), "/tmp/")
			manager.(*fsMetadataManager).fs = mockFS
			manager.(*fsMetadataManager).encoder = mockEncoder

			err := manager.Persist(tt.data)

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

// TestPersistOnDisk stores, reads back and removes the metadata on the
// operating system's file system, where the mocks of TestPersist,
// TestGet and TestClear cannot show where the file lands or with which
// permissions it is written.
func TestPersistOnDisk(t *testing.T) {
	// An absolute path, because one case changes the working directory.
	jobResponseFile, err := filepath.Abs("../internal/jobresponse/testdata/response.json")
	require.NoError(t, err)
	t.Setenv(jobresponse.JobResponseFileVar, jobResponseFile)

	data := Data{
		TaskARN:     "task-arn",
		ContainerIP: "192.168.0.1",
		PrivateKey:  []byte("Testing"),
	}

	tests := map[string]struct {
		// directory returns the configured TaskMetadata directory and
		// the directory the metadata file is expected in.
		directory func(t *testing.T) (configured string, expected string)
		// existingMode is set when the expected directory exists before
		// Persist runs, to the mode it must still have afterwards.
		existingMode os.FileMode
	}{
		"existing directory": {
			directory: func(t *testing.T) (string, string) {
				// Not the 0700 t.TempDir creates, so that a change of
				// mode would show.
				dir := t.TempDir()
				require.NoError(t, os.Chmod(dir, 0o755)) //nolint:gosec // a test directory, opened on purpose
				return dir, dir
			},
			existingMode: 0o755,
		},
		"no directory configured": {
			directory: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				t.Chdir(dir)
				return "", dir
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			configured, expected := tt.directory(t)

			manager := NewMetadataManager(createTestLogger(), configured)
			require.NoError(t, manager.Persist(data))

			if tt.existingMode != 0 {
				info, err := os.Stat(expected)
				require.NoError(t, err)
				assert.Equal(t, tt.existingMode, info.Mode().Perm(), "an existing directory keeps its mode")
			}

			file := filepath.Join(expected, manager.(*fsMetadataManager).filename+".json")
			info, err := os.Stat(file)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

			persisted, err := manager.Get()
			require.NoError(t, err)
			assert.Equal(t, data, persisted)

			require.NoError(t, manager.Clear())
			_, err = os.Stat(file)
			assert.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

func TestGet(t *testing.T) {
	initializeAdapterForTesting(t)
	clear := initializeJobResponseFileForTesting(t)
	defer clear()

	testError := errors.New("simulated error")

	tests := map[string]struct {
		fileExists      bool
		accessFileError error
		readFileError   error
		decodingError   error
		expectedError   error
	}{
		"Retrieve metadata with success": {
			fileExists:      true,
			accessFileError: nil,
			readFileError:   nil,
			decodingError:   nil,
			expectedError:   nil,
		},
		"Error by file not accessible": {
			fileExists:      true,
			accessFileError: testError,
			readFileError:   nil,
			decodingError:   nil,
			expectedError:   testError,
		},
		"Error by file not exist": {
			fileExists:      false,
			accessFileError: nil,
			readFileError:   nil,
			decodingError:   nil,
			expectedError:   os.ErrNotExist,
		},
		"Error when reading file": {
			fileExists:      true,
			accessFileError: nil,
			readFileError:   testError,
			decodingError:   nil,
			expectedError:   testError,
		},
		"Error when decoding JSON": {
			fileExists:      true,
			accessFileError: nil,
			readFileError:   nil,
			decodingError:   testError,
			expectedError:   testError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			mockFS := new(fs.MockFS)
			defer mockFS.AssertExpectations(t)

			mockEncoder := new(encoding.MockEncoder)
			defer mockEncoder.AssertExpectations(t)

			mockFS.On("Exists", mock.AnythingOfType("string")).
				Return(tt.fileExists, tt.accessFileError).
				Once()

			// ReadFile will be called when the file exists and is accessible
			shouldCallReadFile := tt.fileExists && tt.accessFileError == nil
			setExpectationForFSReadFile(mockFS, shouldCallReadFile, tt.readFileError)

			// Encode will be called if no previous errors occurred
			shouldCallEncode := shouldCallReadFile && tt.readFileError == nil
			setExpectationForDecode(mockEncoder, shouldCallEncode, tt.decodingError)

			manager := NewMetadataManager(createTestLogger(), "/tmp/")
			manager.(*fsMetadataManager).fs = mockFS
			manager.(*fsMetadataManager).encoder = mockEncoder

			_, err := manager.Get()

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func setExpectationForFSReadFile(mockFS *fs.MockFS, shouldCall bool, errorToReturn error) {
	if !shouldCall {
		return
	}

	validFileContent := []byte(`{"taskARN":"1234"}`)

	mockFS.On("ReadFile", mock.AnythingOfType("string")).
		Return(validFileContent, errorToReturn).
		Once()
}

func setExpectationForDecode(mockEncoder *encoding.MockEncoder, shouldCall bool, errorToReturn error) {
	if !shouldCall {
		return
	}

	mockEncoder.On("Decode", mock.AnythingOfType("*bytes.Buffer"), mock.Anything).
		Return(errorToReturn).
		Once()
}

func TestClear(t *testing.T) {
	initializeAdapterForTesting(t)
	clear := initializeJobResponseFileForTesting(t)
	defer clear()

	testError := errors.New("simulated error")

	tests := map[string]struct {
		fileExists      bool
		accessFileError error
		removeFileError error
		expectedError   error
	}{
		"Clear with success": {
			fileExists:      true,
			accessFileError: nil,
			removeFileError: nil,
			expectedError:   nil,
		},
		"Error by file not accessible": {
			fileExists:      true,
			accessFileError: testError,
			removeFileError: nil,
			expectedError:   testError,
		},
		"Error by file not exist": {
			fileExists:      false,
			accessFileError: nil,
			removeFileError: nil,
			expectedError:   os.ErrNotExist,
		},
		"Error removing file": {
			fileExists:      true,
			accessFileError: nil,
			removeFileError: testError,
			expectedError:   testError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			mockFS := new(fs.MockFS)
			defer mockFS.AssertExpectations(t)

			mockEncoder := new(encoding.MockEncoder)
			defer mockEncoder.AssertExpectations(t)

			mockFS.On("Exists", mock.AnythingOfType("string")).
				Return(tt.fileExists, tt.accessFileError).
				Once()

			if tt.fileExists && tt.accessFileError == nil {
				mockFS.On("Remove", mock.AnythingOfType("string")).
					Return(tt.removeFileError).
					Once()
			}

			manager := NewMetadataManager(createTestLogger(), "/tmp/")
			manager.(*fsMetadataManager).fs = mockFS
			manager.(*fsMetadataManager).encoder = mockEncoder

			err := manager.Clear()

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}
