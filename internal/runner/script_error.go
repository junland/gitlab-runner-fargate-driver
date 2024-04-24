package runner

import (
	"fmt"
)

type ScriptExecError struct {
	msg string
	err error
}

func NewScriptExecError(err error) *ScriptExecError {
	return &ScriptExecError{
		msg: "script execution error",
		err: err,
	}
}

func (e *ScriptExecError) Format(s fmt.State, _ rune) {
	_, _ = fmt.Fprintf(s, "%s: %v", e.msg, e.err)
}

func (e *ScriptExecError) Error() string {
	return e.msg
}

func (e *ScriptExecError) Is(err error) bool {
	_, ok := err.(*ScriptExecError)

	return ok
}

func (e *ScriptExecError) Unwrap() error {
	return e.err
}
