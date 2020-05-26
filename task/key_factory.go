package task

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
)

// errInvalidPrivateKey will be used to wrap a RSA internal error
type errInvalidPrivateKey struct {
	inner error
}

func (e *errInvalidPrivateKey) Error() string {
	return fmt.Sprintf("invalid private key: %v", e.inner)
}

func (e *errInvalidPrivateKey) Unwrap() error {
	return e.inner
}

func (e *errInvalidPrivateKey) Is(err error) bool {
	_, ok := err.(*errInvalidPrivateKey)
	return ok
}

// SSHKeyFactory is a factory for Public and Private key pairs
type SSHKeyFactory interface {
	MakeKeyPair(bitSize int) (*KeyPair, error)
}

// KeyPair centralizes the generated public and private keys
type KeyPair struct {
	PublicKey  []byte
	PrivateKey []byte
}

type keyFactory struct {
	logger logging.Logger

	// Functions encapsulated to make easier creating unit tests
	generateRSAKey  func(random io.Reader, bits int) (*rsa.PrivateKey, error)
	newSSHPublicKey func(key interface{}) (ssh.PublicKey, error)
}

// NewSSHKeyFactory instantiates a concrete instance of SSHKeyFactory
func NewSSHKeyFactory(logger logging.Logger) SSHKeyFactory {
	return &keyFactory{
		logger:          logger,
		generateRSAKey:  rsa.GenerateKey,
		newSSHPublicKey: ssh.NewPublicKey,
	}
}

// MakeKeyPair generates a RSA Public and Private key pair
func (r *keyFactory) MakeKeyPair(bitSize int) (*KeyPair, error) {
	r.logger.Debug("[MakeKeyPair] Will generate new key pair")

	privateKey, err := r.generateRSAKey(rand.Reader, bitSize)
	if err != nil {
		return nil, fmt.Errorf("generating the private key: %w", err)
	}

	err = privateKey.Validate()
	if err != nil {
		return nil, &errInvalidPrivateKey{inner: err}
	}

	privateKeyBytes := r.encodePrivateKeyToPEM(privateKey)

	publicKeyBytes, err := r.getPublicKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("generating the public key: %w", err)
	}

	keyPair := &KeyPair{
		PublicKey:  publicKeyBytes,
		PrivateKey: privateKeyBytes,
	}

	r.logger.Debug("[MakeKeyPair] Key pair generated with success")

	return keyPair, nil
}

func (r *keyFactory) encodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   x509.MarshalPKCS1PrivateKey(privateKey),
	}

	return pem.EncodeToMemory(&privBlock)
}

func (r *keyFactory) getPublicKey(privateKey *rsa.PrivateKey) ([]byte, error) {
	publicKey, err := r.newSSHPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	return ssh.MarshalAuthorizedKey(publicKey), nil
}
