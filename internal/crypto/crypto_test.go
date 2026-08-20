package crypto

import (
	"bytes"
	"errors"
	"testing"
)

func testParams() KDFParams { return KDFParams{MemoryKiB: 1024, Time: 1, Threads: 1} }

// validEnvelope returns an envelope that passes every validateEnvelope check, so
// mutating one field isolates the check under test.
func validEnvelope(t *testing.T, passphrase []byte) Envelope {
	t.Helper()
	envelope, err := Encrypt(passphrase, []byte("payload"), PurposeGeneral, testParams())
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	envelope, err := Encrypt([]byte("correct horse battery staple"), []byte("environment data"), PurposeGeneral, testParams())
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := Decrypt([]byte("correct horse battery staple"), &envelope, PurposeGeneral)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plaintext, []byte("environment data")) {
		t.Fatal("plaintext changed after round trip")
	}
}

func TestEncryptDecryptEmptyPlaintext(t *testing.T) {
	passphrase := []byte("passphrase")
	envelope, err := Encrypt(passphrase, nil, PurposeGeneral, testParams())
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := Decrypt(passphrase, &envelope, PurposeGeneral)
	if err != nil {
		t.Fatal(err)
	}
	if len(plaintext) != 0 {
		t.Fatalf("expected empty plaintext, got %q", plaintext)
	}
}

func TestEncryptGeneratesFreshSaltAndNonce(t *testing.T) {
	passphrase, plaintext := []byte("passphrase"), []byte("same text")
	first, err := Encrypt(passphrase, plaintext, PurposeGeneral, testParams())
	if err != nil {
		t.Fatal(err)
	}
	second, err := Encrypt(passphrase, plaintext, PurposeGeneral, testParams())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first.Salt, second.Salt) {
		t.Fatal("salt must be freshly generated per encryption")
	}
	if bytes.Equal(first.Nonce, second.Nonce) {
		t.Fatal("nonce must be freshly generated per encryption")
	}
	if bytes.Equal(first.Ciphertext, second.Ciphertext) {
		t.Fatal("identical inputs must not produce identical ciphertext")
	}
}

func TestEncryptRejectsInvalidInput(t *testing.T) {
	passphrase := []byte("passphrase")
	for _, tc := range []struct {
		name       string
		passphrase []byte
		purpose    KeyPurpose
		params     KDFParams
		wrapped    error
	}{
		{name: "empty passphrase", passphrase: nil, purpose: PurposeGeneral, params: testParams()},
		{name: "unknown purpose", passphrase: passphrase, purpose: KeyPurpose("archive"), params: testParams(), wrapped: ErrInvalidEnvelope},
		{name: "memory below minimum", passphrase: passphrase, purpose: PurposeGeneral, params: KDFParams{MemoryKiB: 1023, Time: 1, Threads: 1}, wrapped: ErrInvalidEnvelope},
		{name: "memory above maximum", passphrase: passphrase, purpose: PurposeGeneral, params: KDFParams{MemoryKiB: 1048577, Time: 1, Threads: 1}, wrapped: ErrInvalidEnvelope},
		{name: "zero time", passphrase: passphrase, purpose: PurposeGeneral, params: KDFParams{MemoryKiB: 1024, Time: 0, Threads: 1}, wrapped: ErrInvalidEnvelope},
		{name: "time above maximum", passphrase: passphrase, purpose: PurposeGeneral, params: KDFParams{MemoryKiB: 1024, Time: 11, Threads: 1}, wrapped: ErrInvalidEnvelope},
		{name: "zero threads", passphrase: passphrase, purpose: PurposeGeneral, params: KDFParams{MemoryKiB: 1024, Time: 1, Threads: 0}, wrapped: ErrInvalidEnvelope},
		{name: "threads above maximum", passphrase: passphrase, purpose: PurposeGeneral, params: KDFParams{MemoryKiB: 1024, Time: 1, Threads: 33}, wrapped: ErrInvalidEnvelope},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Encrypt(tc.passphrase, []byte("payload"), tc.purpose, tc.params)
			if err == nil {
				t.Fatal("expected encryption to fail")
			}
			if tc.wrapped != nil && !errors.Is(err, tc.wrapped) {
				t.Fatalf("expected error wrapping %v, got %v", tc.wrapped, err)
			}
		})
	}
}

func TestDecryptRejectsEmptyPassphrase(t *testing.T) {
	envelope := validEnvelope(t, []byte("passphrase"))
	if _, err := Decrypt(nil, &envelope, PurposeGeneral); err == nil {
		t.Fatal("expected decryption to fail")
	}
}

func TestDecryptRejectsMalformedEnvelope(t *testing.T) {
	passphrase := []byte("passphrase")
	for _, tc := range []struct {
		name   string
		mutate func(*Envelope)
	}{
		{"unsupported version", func(e *Envelope) { e.Version = EnvelopeVersion + 1 }},
		{"unknown algorithm", func(e *Envelope) { e.Algorithm = "chacha20-poly1305" }},
		{"unknown purpose", func(e *Envelope) { e.Purpose = KeyPurpose("archive") }},
		{"short salt", func(e *Envelope) { e.Salt = e.Salt[:saltLength-1] }},
		{"long salt", func(e *Envelope) { e.Salt = append(e.Salt, 0) }},
		{"invalid kdf params", func(e *Envelope) { e.KDF.Time = 0 }},
		{"short nonce", func(e *Envelope) { e.Nonce = e.Nonce[:len(e.Nonce)-1] }},
		{"long nonce", func(e *Envelope) { e.Nonce = append(e.Nonce, 0) }},
		{"ciphertext shorter than tag", func(e *Envelope) { e.Ciphertext = e.Ciphertext[:15] }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			envelope := validEnvelope(t, passphrase)
			tc.mutate(&envelope)
			if _, err := Decrypt(passphrase, &envelope, PurposeGeneral); !errors.Is(err, ErrInvalidEnvelope) {
				t.Fatalf("expected error wrapping %v, got %v", ErrInvalidEnvelope, err)
			}
		})
	}
}

func TestDecryptRejectsTamperingWrongPassphraseAndPurpose(t *testing.T) {
	passphrase := []byte("correct horse battery staple")
	envelope, err := Encrypt(passphrase, []byte("secret"), PurposeCredential, testParams())
	if err != nil {
		t.Fatal(err)
	}
	tampered := envelope
	tampered.Ciphertext = append([]byte(nil), envelope.Ciphertext...)
	tampered.Ciphertext[0] ^= 1
	for _, tc := range []struct {
		name       string
		passphrase []byte
		envelope   Envelope
		purpose    KeyPurpose
		wrapped    error
	}{
		{"tampered ciphertext", passphrase, tampered, PurposeCredential, ErrAuthentication},
		{"wrong passphrase", []byte("wrong"), envelope, PurposeCredential, ErrAuthentication},
		{"purpose mismatch", passphrase, envelope, PurposeGeneral, ErrInvalidEnvelope},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Decrypt(tc.passphrase, &tc.envelope, tc.purpose); !errors.Is(err, tc.wrapped) {
				t.Fatalf("expected error wrapping %v, got %v", tc.wrapped, err)
			}
		})
	}
}

func TestPurposeDerivesIndependentCiphertextAndHashVerification(t *testing.T) {
	passphrase, plaintext := []byte("passphrase"), []byte("same text")
	general, err := Encrypt(passphrase, plaintext, PurposeGeneral, testParams())
	if err != nil {
		t.Fatal(err)
	}
	credential, err := Encrypt(passphrase, plaintext, PurposeCredential, testParams())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(general.Ciphertext, credential.Ciphertext) {
		t.Fatal("key purposes must not share ciphertext")
	}
	hash := Hash(plaintext)
	if !VerifyHash(plaintext, hash) || VerifyHash([]byte("changed"), hash) {
		t.Fatal("hash verification mismatch")
	}
}
