package runner

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScriptExitCodeFailureFormatAndExitCode(t *testing.T) {
	err := NewScriptExitCodeError(fmt.Errorf("Process exited with status 143 from signal TERM"), 143)
	output := fmt.Sprintf("%v", err)

	assert.Equal(t, "script exit code 143: Process exited with status 143 from signal TERM", output)
	assert.Equal(t, 143, err.ExitStatus())
}
