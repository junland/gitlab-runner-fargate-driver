package task

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/assertions"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/encoding"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/fs"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging/test"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/runner"
)

func TestNewMetadataManager(t *testing.T) {
	runner.InitAdapter()

	manager := NewMetadataManager(createTestLogger(), "/tmp/")
	assert.NotNil(t, manager, "instance should have been created")
}

func createTestLogger() logging.Logger {
	return test.NewNullLogger()
}

func TestPersist(t *testing.T) {
	runner.InitAdapter()

	testError := errors.New("simulated error")

	tests := map[string]struct {
		marshallingError error
		writeFileError   error
		expectedError    error
	}{
		"Persist with success": {
			marshallingError: nil,
			writeFileError:   nil,
			expectedError:    nil,
		},
		"Error on marshalling": {
			marshallingError: testError,
			writeFileError:   nil,
			expectedError:    testError,
		},
		"Error on persisting file": {
			marshallingError: nil,
			writeFileError:   testError,
			expectedError:    testError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			mockFS := new(fs.MockFS)
			defer mockFS.AssertExpectations(t)

			mockEncoder := new(encoding.MockEncoder)
			defer mockEncoder.AssertExpectations(t)

			mockEncoder.On("Encode",
				mock.AnythingOfType("FargateTaskData"),
				mock.AnythingOfType("*bytes.Buffer"),
			).
				Return(tt.marshallingError).
				Once()

			if tt.marshallingError == nil {
				mockFS.On("WriteFile",
					mock.AnythingOfType("string"),
					mock.AnythingOfType("[]uint8"),
					mock.AnythingOfType("os.FileMode"),
				).
					Return(tt.writeFileError).
					Once()
			}

			manager := NewMetadataManager(createTestLogger(), "/tmp/")
			manager.(*fsMetadataManager).fs = mockFS
			manager.(*fsMetadataManager).encoder = mockEncoder

			fargateTaskData := FargateTaskData{}
			err := manager.Persist(fargateTaskData)

			if tt.expectedError != nil {
				assertions.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestGet(t *testing.T) {
	runner.InitAdapter()

	testError := errors.New("simulated error")

	tests := map[string]struct {
		fileExists         bool
		accessFileError    error
		readFileError      error
		unmarshallingError error
		expectedError      error
	}{
		"Retrieve metadata with success": {
			fileExists:         true,
			accessFileError:    nil,
			readFileError:      nil,
			unmarshallingError: nil,
			expectedError:      nil,
		},
		"Error by file not accessible": {
			fileExists:         true,
			accessFileError:    testError,
			readFileError:      nil,
			unmarshallingError: nil,
			expectedError:      testError,
		},
		"Error by file not exist": {
			fileExists:         false,
			accessFileError:    nil,
			readFileError:      nil,
			unmarshallingError: nil,
			expectedError:      ErrFileNotExists,
		},
		"Error when reading file": {
			fileExists:         true,
			accessFileError:    nil,
			readFileError:      testError,
			unmarshallingError: nil,
			expectedError:      testError,
		},
		"Error when unmarshalling": {
			fileExists:         true,
			accessFileError:    nil,
			readFileError:      nil,
			unmarshallingError: testError,
			expectedError:      testError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			mockFS := new(fs.MockFS)
			defer mockFS.AssertExpectations(t)

			mockEncoder := new(encoding.MockEncoder)
			defer mockEncoder.AssertExpectations(t)

			mockFS.On("Exists",
				mock.AnythingOfType("string"),
			).
				Return(tt.fileExists, tt.accessFileError).
				Once()

			// ReadFile will be called when the file exists and is accessible
			shouldCallReadFile := tt.fileExists && tt.accessFileError == nil
			setExpectationForFSReadFile(mockFS, shouldCallReadFile, tt.readFileError)

			// Encode will be called if no previous errors occurred
			shouldCallEncode := shouldCallReadFile && tt.readFileError == nil
			setExpectationForDecode(mockEncoder, shouldCallEncode, tt.unmarshallingError)

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

	mockFS.On("ReadFile",
		mock.AnythingOfType("string"),
	).
		Return(validFileContent, errorToReturn).
		Once()
}

func setExpectationForDecode(mockEncoder *encoding.MockEncoder, shouldCall bool, errorToReturn error) {
	if !shouldCall {
		return
	}

	mockEncoder.On("Decode",
		mock.AnythingOfType("*bytes.Buffer"),
		mock.Anything,
	).
		Return(errorToReturn).
		Once()
}

func TestClear(t *testing.T) {
	runner.InitAdapter()

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
			expectedError:   ErrFileNotExists,
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

			mockFS.On("Exists",
				mock.AnythingOfType("string"),
			).
				Return(tt.fileExists, tt.accessFileError).
				Once()

			if tt.fileExists && tt.accessFileError == nil {
				mockFS.On("Remove",
					mock.AnythingOfType("string"),
				).
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
