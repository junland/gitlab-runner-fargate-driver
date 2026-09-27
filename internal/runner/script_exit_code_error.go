package runner

import "fmt"

type ScriptExitCodeError struct {
	status int
	msg    string
}

func NewScriptExitCodeError(msg string, status int) *ScriptExitCodeError {
	return &ScriptExitCodeError{
		msg:    msg,
		status: status,
	}
}

func (e *ScriptExitCodeError) ExitStatus() int {
	return e.status
}

func (e *ScriptExitCodeError) Error() string {
	return fmt.Sprintf("Script exited with non-zero status: %v", e.msg)
}
