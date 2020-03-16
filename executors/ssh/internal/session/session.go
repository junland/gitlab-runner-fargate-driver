package session

import (
	"golang.org/x/crypto/ssh"
)

type Session interface {
	ExecuteScript(script string) error
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

func (s *defaultSession) ExecuteScript(script string) error {
	return s.internal.Run(script)
}

func (s *defaultSession) Close() {
	_ = s.internal.Close()
}
