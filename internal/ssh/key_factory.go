package ssh

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/logging"
)

// ErrInvalidPrivateKey will be used to wrap an ecdsa internal error
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
	generateECDSAKey func(c elliptic.Curve, rand io.Reader) (*ecdsa.PrivateKey, error)
	newSSHPublicKey    func(key interface{}) (ssh.PublicKey, error)
}

// NewKeyFactory instantiates a concrete instance of KeyFactory
func NewKeyFactory(logger logging.Logger) KeyFactory {
	return &keyFactory{
		logger:             logger,
		generateECDSAKey: ecdsa.GenerateKey,
		newSSHPublicKey:    ssh.NewPublicKey,
	}
}

// Create generates an ecdsa Public and Private key pair
func (r *keyFactory) Create() (*KeyPair, error) {
	r.logger.Debug("[Create] Will generate new key pair")
    privateKey, err := r.generateECDSAKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating the private and public key: %w", err)
	}
    var (
        bytes     []byte
        publicKey ssh.PublicKey
    )

	publicKey, err = ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		return nil, fmt.Errorf("generating the private and public key: %w", err)
	}

    pubBytes := ssh.MarshalAuthorizedKey(publicKey)

    bytes, err = x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("converting the the private key to PKCS8 format: %w", err)
	}

    privBytes := pem.EncodeToMemory(&pem.Block{
        Type: "EC PRIVATE KEY",
        Bytes: bytes,
    })


	keyPair := &KeyPair{
		PublicKey:  pubBytes,
		PrivateKey: privBytes,
	}

	r.logger.Debug("[Create] Key pair generated with success")

	return keyPair, nil
}