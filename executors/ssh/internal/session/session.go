package session

import (
	"context"
	"fmt"

	"golang.org/x/crypto/ssh"
)

type Session interface {
	ExecuteScript(ctx context.Context, script string) error
	Close()
}

func New(s *ssh.Session) Session {
	return &defaultSession{
		internal: s,
	}
}

type defaultSession struct {
	internal *ssh.Session
}

func (s *defaultSession) ExecuteScript(ctx context.Context, script string) error {
	waitErr := make(chan error)

	go func() {
		waitErr <- s.internal.Run(script)
	}()

	select {
	case err := <-waitErr:
		return fmt.Errorf("executing SSH command: %w", err)
	case <-ctx.Done():
		err := s.internal.Signal(ssh.SIGTERM)
		return fmt.Errorf("killing SSH command: %w", err)
	}
}

func (s *defaultSession) Close() {
	_ = s.internal.Close()
}
