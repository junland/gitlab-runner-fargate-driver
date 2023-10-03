package task

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/assertions"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/encoding"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/fs"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/runner"
)

func TestNewMetadataManager(t *testing.T) {
	initializeAdapterForTesting(t)

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

func createTestLogger() logging.Logger {
	return test.NewNullLogger()
}

func TestPersist(t *testing.T) {
	initializeAdapterForTesting(t)

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

func TestGet(t *testing.T) {
	initializeAdapterForTesting(t)

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

	// ensure Clear is a no-op
	assert.NoError(t, jrfmm.Clear())
	assert.FileExists(t, jobResponseFileName)
}
