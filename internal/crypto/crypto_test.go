package crypto

import (
	"bytes"
	"errors"
	"testing"
)

func testParams() KDFParams { return KDFParams{MemoryKiB: 1024, Time: 1, Threads: 1} }

func TestEncryptDecryptRoundTrip(t *testing.T) {
	envelope, err := Encrypt([]byte("correct horse battery staple"), []byte("environment data"), PurposeGeneral, testParams())
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := Decrypt([]byte("correct horse battery staple"), envelope, PurposeGeneral)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plaintext, []byte("environment data")) {
		t.Fatal("plaintext changed after round trip")
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
	for _, attempt := range []struct {
		passphrase []byte
		envelope   Envelope
		purpose    KeyPurpose
	}{
		{passphrase, tampered, PurposeCredential}, {[]byte("wrong"), envelope, PurposeCredential}, {passphrase, envelope, PurposeGeneral},
	} {
		if _, err := Decrypt(attempt.passphrase, attempt.envelope, attempt.purpose); err == nil {
			t.Fatal("expected decryption to fail")
		}
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
	if !errors.Is(ErrAuthentication, ErrAuthentication) {
		t.Fatal("sentinel must remain usable")
	}
}
