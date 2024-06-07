package runner

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var errTestBuild = errors.New("test error")

func TestErrBuildFailureError(t *testing.T) {
	err := NewBuildFailureError(errTestBuild)
	assert.EqualError(t, err, "build failure")
}

func TestErrBuildFailureFormat(t *testing.T) {
	err := NewBuildFailureError(errTestBuild)
	output := fmt.Sprintf("%v", err)

	assert.Equal(t, "build failure: test error", output)
}

func TestErrBuildFailureIs(t *testing.T) {
	err := NewBuildFailureError(errTestBuild)

	assert.True(t, err.Is(&BuildFailureError{}))
	assert.False(t, err.Is(errors.New("test")))
}

func TestErrBuildFailureUnwrap(t *testing.T) {
	err := NewBuildFailureError(errTestBuild)

	assert.Equal(t, errTestBuild, err.Unwrap())
}
