package test

import (
	"bytes"
	"fmt"
	"io"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
)

func NewNullLogger() logging.Logger {
	logger := logging.New()
	_ = logger.SetFormat(logging.FormatTextSimple)
	logger.SetOutput(io.Discard)

	return logger
}

func NewBufferedLogger() (logging.Logger, fmt.Stringer) {
	buf := new(bytes.Buffer)

	logger := logging.New()
	_ = logger.SetFormat(logging.FormatTextSimple)
	logger.SetOutput(buf)

	return logger, buf
}
