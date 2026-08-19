// Package crypto provides the S9 cryptographic foundation for AERS.
// It owns password-based key derivation, authenticated encryption, and hashing.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	EnvelopeVersion uint8  = 1
	AES256GCM              = "aes-256-gcm"
	keyLength       uint32 = 32
	saltLength             = 16
)

var (
	ErrInvalidEnvelope = errors.New("invalid encrypted envelope")
	ErrAuthentication  = errors.New("ciphertext authentication failed")
)

// KeyPurpose makes derived material domain-specific within a single passphrase.
type KeyPurpose string

const (
	PurposeGeneral    KeyPurpose = "general"
	PurposeCredential KeyPurpose = "credential"
)

// KDFParams are Argon2id work factors. Memory is measured in KiB.
type KDFParams struct {
	MemoryKiB uint32
	Time      uint32
	Threads   uint8
}

func DefaultKDFParams() KDFParams {
	return KDFParams{MemoryKiB: 65536, Time: 3, Threads: 1}
}

func (p KDFParams) validate() error {
	if p.MemoryKiB < 1024 || p.MemoryKiB > 1048576 {
		return fmt.Errorf("%w: memory must be between 1024 and 1048576 KiB", ErrInvalidEnvelope)
	}
	if p.Time == 0 || p.Time > 10 {
		return fmt.Errorf("%w: time must be between 1 and 10", ErrInvalidEnvelope)
	}
	if p.Threads == 0 || p.Threads > 32 {
		return fmt.Errorf("%w: threads must be between 1 and 32", ErrInvalidEnvelope)
	}
	return nil
}

// Envelope is a transport-safe, self-describing encrypted payload. Its fields
// contain no passphrase or derived key material.
type Envelope struct {
	Version    uint8
	Algorithm  string
	Purpose    KeyPurpose
	KDF        KDFParams
	Salt       []byte
	Nonce      []byte
	Ciphertext []byte
}

// Encrypt derives a fresh key for purpose and returns AES-256-GCM ciphertext.
func Encrypt(passphrase, plaintext []byte, purpose KeyPurpose, params KDFParams) (Envelope, error) {
	if len(passphrase) == 0 {
		return Envelope{}, errors.New("passphrase cannot be empty")
	}
	if err := params.validate(); err != nil {
		return Envelope{}, err
	}
	if !validPurpose(purpose) {
		return Envelope{}, fmt.Errorf("%w: unknown key purpose", ErrInvalidEnvelope)
	}
	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return Envelope{}, fmt.Errorf("generate salt: %w", err)
	}
	key := deriveKey(passphrase, salt, purpose, params)
	defer zero(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return Envelope{}, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Envelope{}, fmt.Errorf("create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return Envelope{}, fmt.Errorf("generate nonce: %w", err)
	}
	return Envelope{Version: EnvelopeVersion, Algorithm: AES256GCM, Purpose: purpose, KDF: params, Salt: salt, Nonce: nonce, Ciphertext: gcm.Seal(nil, nonce, plaintext, []byte(purpose))}, nil
}

// Decrypt authenticates the envelope before returning plaintext.
func Decrypt(passphrase []byte, envelope Envelope, expectedPurpose KeyPurpose) ([]byte, error) {
	if len(passphrase) == 0 {
		return nil, errors.New("passphrase cannot be empty")
	}
	if err := validateEnvelope(envelope, expectedPurpose); err != nil {
		return nil, err
	}
	key := deriveKey(passphrase, envelope.Salt, envelope.Purpose, envelope.KDF)
	defer zero(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	plaintext, err := gcm.Open(nil, envelope.Nonce, envelope.Ciphertext, []byte(envelope.Purpose))
	if err != nil {
		return nil, ErrAuthentication
	}
	return plaintext, nil
}

func validateEnvelope(e Envelope, expected KeyPurpose) error {
	if e.Version != EnvelopeVersion || e.Algorithm != AES256GCM || !validPurpose(e.Purpose) || e.Purpose != expected || len(e.Salt) != saltLength {
		return ErrInvalidEnvelope
	}
	if err := e.KDF.validate(); err != nil {
		return err
	}
	if len(e.Nonce) != 12 || len(e.Ciphertext) < 16 {
		return ErrInvalidEnvelope
	}
	return nil
}

func validPurpose(p KeyPurpose) bool { return p == PurposeGeneral || p == PurposeCredential }
func deriveKey(passphrase, salt []byte, purpose KeyPurpose, params KDFParams) []byte {
	// Purpose is incorporated into the password input in addition to GCM AAD.
	input := make([]byte, 0, len(passphrase)+1+len(purpose))
	input = append(input, passphrase...)
	input = append(input, 0)
	input = append(input, purpose...)
	defer zero(input)
	return argon2.IDKey(input, salt, params.Time, params.MemoryKiB, params.Threads, keyLength)
}

func zero(data []byte) {
	for i := range data {
		data[i] = 0
	}
}

// Hash returns a SHA-256 digest for an artifact or archive payload.
func Hash(data []byte) [sha256.Size]byte { return sha256.Sum256(data) }

// VerifyHash reports whether data matches an expected SHA-256 digest.
func VerifyHash(data []byte, expected [sha256.Size]byte) bool { return Hash(data) == expected }
