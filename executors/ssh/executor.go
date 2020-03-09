package ssh

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/ssh"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/executors"
	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
)

// ErrNotConnected is return when a previous connection was not established
var ErrNotConnected = errors.New("not connected to server")

type executor struct {
	client *ssh.Client
	logger logging.Logger

	// Some functions were encapsulated to make easier creating unit tests for this module
	internalDial          func(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error)
	internalCreateSession func(client *ssh.Client) (*ssh.Session, error)
	internalExecuteScript func(session *ssh.Session, script string) error
	internalCloseSession  func(session *ssh.Session)
	internalDisconnect    func(client *ssh.Client) error
}

// NewExecutor is the constructor for an instance of the executor interface
func NewExecutor(logger logging.Logger) executors.Executor {
	executor := &executor{}
	executor.logger = logger

	executor.internalDial = ssh.Dial
	executor.internalCreateSession = func(client *ssh.Client) (*ssh.Session, error) {
		return client.NewSession()
	}
	executor.internalExecuteScript = func(session *ssh.Session, script string) error {
		return session.Run(script)
	}
	executor.internalCloseSession = func(session *ssh.Session) {
		session.Close()
	}
	executor.internalDisconnect = func(client *ssh.Client) error {
		return client.Close()
	}

	return executor
}

func (s *executor) Execute(connection executors.ConnectionSettings, script []byte) (err error) {
	s.logger.Debug("[Execute] Will connect to server and execute the specified shell script")

	err = s.connectToServer(
		connection.Hostname, connection.Port, connection.Username, connection.PrivateKey,
	)
	if err != nil {
		return fmt.Errorf("error connecting to server: %w", err)
	}

	// Use a defer function to ensure disconnection after executing the script
	defer func() {
		ferr := s.disconnect()
		if ferr != nil && err == nil {
			err = fmt.Errorf("error disconnecting from server: %w", ferr)
		}
	}()

	err = s.executeScript(script, os.Stdout, os.Stderr)
	if err != nil {
		return fmt.Errorf("error executing script: %w", err)
	}

	s.logger.Debug("[Execute] Successfully executed script")

	return nil
}

func (s *executor) connectToServer(hostname string, port int, username string, privateKey []byte) error {
	s.logger.Debug("[connectToServer] Will connect to server via SSH")

	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("error parsing private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%d", hostname, port)
	client, err := s.internalDial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("error connecting to server: %w", err)
	}

	s.client = client

	s.logger.Debug("[connectToServer] Successfully connected to server")

	return nil
}

func (s *executor) disconnect() error {
	s.logger.Debug("[disconnect] Will disconnect from server")

	if s.client == nil {
		return nil
	}

	err := s.internalDisconnect(s.client)
	if err != nil {
		return fmt.Errorf("error disconnecting from server: %w", err)
	}

	s.client = nil

	s.logger.Debug("[disconnect] Successfully disconnected from server")

	return nil
}

func (s *executor) executeScript(script []byte, stdout io.Writer, stderr io.Writer) error {
	s.logger.Debug("[executeScript] Will execute a remote script")

	if s.client == nil {
		return fmt.Errorf("could not execute the script: %w", ErrNotConnected)
	}

	session, err := s.internalCreateSession(s.client)
	if err != nil {
		return fmt.Errorf("error creating session for ssh client: %w", err)
	}
	defer s.internalCloseSession(session)

	session.Stdout = stdout
	session.Stderr = stderr

	err = s.internalExecuteScript(session, string(script))
	if err != nil {
		return fmt.Errorf("error executing remote script: %w", err)
	}

	s.logger.Debug("[executeScript] Command executed")

	return nil
}
