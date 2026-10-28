package trust

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

// GeneratePEM creates a fresh offline signing key pair, without writing files.
func GeneratePEM() (privatePEM, publicPEM []byte, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	encodedPrivate, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	encodedPublic, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedPrivate}),
		pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encodedPublic}), nil
}
func ParsePrivatePEM(content []byte) (ed25519.PrivateKey, error) {
	block, rest := pem.Decode(content)
	if block == nil || block.Type != "PRIVATE KEY" || len(rest) != 0 {
		return nil, errors.New("invalid PKCS8 private key PEM")
	}
	decoded, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	key, ok := decoded.(ed25519.PrivateKey)
	if !ok || len(key) != ed25519.PrivateKeySize {
		return nil, errors.New("expected Ed25519 private key")
	}
	return key, nil
}
func ParsePublicPEM(content []byte) (ed25519.PublicKey, error) {
	block, rest := pem.Decode(content)
	if block == nil || block.Type != "PUBLIC KEY" || len(rest) != 0 {
		return nil, errors.New("invalid public key PEM")
	}
	decoded, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	key, ok := decoded.(ed25519.PublicKey)
	if !ok || len(key) != ed25519.PublicKeySize {
		return nil, errors.New("expected Ed25519 public key")
	}
	return key, nil
}
