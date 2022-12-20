package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
)

const (
	ed25519PrivateKeyType = "PRIVATE KEY"
)

// ErrInvalidPrivateKey will be used to wrap an Ed25519 internal error
type ErrInvalidPrivateKey struct {
	inner error
}

func (e *ErrInvalidPrivateKey) Error() string {
	return fmt.Sprintf("invalid private key: %v", e.inner)
}

func (e *ErrInvalidPrivateKey) Unwrap() error {
	return e.inner
}

func (e *ErrInvalidPrivateKey) Is(err error) bool {
	_, ok := err.(*ErrInvalidPrivateKey)
	return ok
}

// KeyFactory is a factory for Public and Private key pairs
type KeyFactory interface {
	Create() (*KeyPair, error)
}

// KeyPair centralizes the generated public and private keys
type KeyPair struct {
	PublicKey  []byte
	PrivateKey []byte
}

type keyFactory struct {
	logger logging.Logger

	// Functions encapsulated to make easier creating unit tests
	generateEd25519Key func(rand io.Reader) (ed25519.PublicKey, ed25519.PrivateKey, error)
	newSSHPublicKey    func(key interface{}) (ssh.PublicKey, error)
}

// NewKeyFactory instantiates a concrete instance of KeyFactory
func NewKeyFactory(logger logging.Logger) KeyFactory {
	return &keyFactory{
		logger:             logger,
		generateEd25519Key: ed25519.GenerateKey,
		newSSHPublicKey:    ssh.NewPublicKey,
	}
}

// Create generates an ed25519 Public and Private key pair
func (r *keyFactory) Create() (*KeyPair, error) {
	r.logger.Debug("[Create] Will generate new key pair")

	publicKey, privateKey, err := r.generateEd25519Key(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating the private and public key: %w", err)
	}

	// Only helps with existing Test Mocks
	if !publicKey.Equal(privateKey.Public()) {
		err = errors.New("private Key is not valid for public Key")
		return nil, &ErrInvalidPrivateKey{inner: err}
	}

	privateKeyBytes, err := r.encodePrivateKeyToPEM(privateKey)
	if err != nil {
		return nil, fmt.Errorf("converting the the private key to PKCS8 format: %w", err)
	}

	publicKeyBytes, err := r.getPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("generating the ssh public key: %w", err)
	}

	keyPair := &KeyPair{
		PublicKey:  publicKeyBytes,
		PrivateKey: privateKeyBytes,
	}

	r.logger.Debug("[Create] Key pair generated with success")

	return keyPair, nil
}

func (r *keyFactory) encodePrivateKeyToPEM(privateKey ed25519.PrivateKey) ([]byte, error) {
	privDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	block := pem.Block{
		Type:    ed25519PrivateKeyType,
		Headers: nil,
		Bytes:   privDER,
	}

	return pem.EncodeToMemory(&block), nil
}

func (r *keyFactory) getPublicKey(pubKey ed25519.PublicKey) ([]byte, error) {
	publicKey, err := r.newSSHPublicKey(pubKey)
	if err != nil {
		return nil, err
	}

	return ssh.MarshalAuthorizedKey(publicKey), nil
}
