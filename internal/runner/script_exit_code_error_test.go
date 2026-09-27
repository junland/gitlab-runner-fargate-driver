package runner

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScriptExitCodeFailureFormatAndExitCode(t *testing.T) {
	err := NewScriptExitCodeError("Process exited with status 143 from signal TERM", 143)
	output := fmt.Sprintf("%v", err)

	assert.Equal(t, "Script exited with non-zero status: Process exited with status 143 from signal TERM", output)
	assert.Equal(t, 143, err.ExitStatus())
}
