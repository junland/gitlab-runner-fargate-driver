package runner

import (
	"fmt"
)

type ScriptExitCodeError struct {
	status int
	msg    string
	signal string
}

func NewScriptExitCodeError(msg string, status int, signal string) *ScriptExitCodeError {
	return &ScriptExitCodeError{
		msg:    msg,
		status: status,
		signal: signal,
	}
}

func (e *ScriptExitCodeError) ExitStatus() int {
	return e.status
}

func (e *ScriptExitCodeError) Signal() string {
	return e.signal
}

func (e *ScriptExitCodeError) Msg() string {
	return e.msg
}

func (e *ScriptExitCodeError) Error() string {
	return e.String()
}

func (e *ScriptExitCodeError) String() string {
	str := fmt.Sprintf("Script exited with status %v", e.status)
	if e.signal != "" {
		str += fmt.Sprintf(" from signal %v", e.signal)
	}
	if e.msg != "" {
		str += fmt.Sprintf(". Reason was: %v", e.msg)
	}
	return str
}
