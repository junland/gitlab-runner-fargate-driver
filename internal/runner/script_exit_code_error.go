package runner

import "fmt"

type ScriptExitCodeError struct {
	status int
	err    error
}

func NewScriptExitCodeError(err error, status int) *ScriptExitCodeError {
	return &ScriptExitCodeError{
		status: status,
		err:    err,
	}
}

func (e *ScriptExitCodeError) ExitStatus() int { return e.status }
func (e *ScriptExitCodeError) Unwrap() error   { return e.err }

func (e *ScriptExitCodeError) Error() string {
	return fmt.Sprintf("script exit code %d: %v", e.status, e.err)
}
