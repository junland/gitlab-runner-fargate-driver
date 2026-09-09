package runner

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScriptExitCodeFailureFormat(t *testing.T) {
	err := NewScriptExitCodeError("exit-signal", 143, "TERM")
	output := fmt.Sprintf("%v", err)

	assert.Equal(t, "Script exited with status 143 from signal TERM. Reason was: exit-signal", output)
}
