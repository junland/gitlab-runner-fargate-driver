package runner

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var errTestScript = errors.New("test error")

func TestErrScriptFailureError(t *testing.T) {
	err := NewScriptExecError(errTestScript)
	assert.EqualError(t, err, "script execution error")
}

func TestErrScriptFailureFormat(t *testing.T) {
	err := NewScriptExecError(errTestScript)
	output := fmt.Sprintf("%v", err)

	assert.Equal(t, "script execution error: test error", output)
}

func TestErrScriptFailureIs(t *testing.T) {
	err := NewScriptExecError(errTestScript)

	assert.True(t, err.Is(&ScriptExecError{}))
	assert.False(t, err.Is(errors.New("test")))
}

func TestErrScriptFailureUnwrap(t *testing.T) {
	err := NewScriptExecError(errTestScript)

	assert.Equal(t, errTestScript, err.Unwrap())
}
