# Title

Crypto Envelope and Key Separation

# Status

Accepted

# Context

S9 must encrypt every archive before storage, derive keys from an operation-time
master passphrase, and give credential-isolated data separate cryptographic
material. The implementation must also reject corrupted or tampered ciphertext
before restore can proceed (NFR-1.1 through NFR-1.8).

# Decision

The S9 foundation uses Argon2id to derive a 32-byte AES-256-GCM key. Each
encryption operation generates a fresh random salt and nonce. The resulting
envelope contains a format version, KDF parameters, salt, nonce, ciphertext,
and a key purpose. The purpose is included as AES-GCM additional authenticated
data, so a general-data envelope cannot be substituted for a credential
envelope.

The credential segment uses the same user-provided master passphrase but a
distinct `credential` purpose and fresh salt, producing independent derived
key material. Passphrases and derived keys are accepted only as operation-time
byte slices and are not persisted by S9.

KDF parameters are validated against conservative bounds before derivation to
avoid malformed envelope inputs exhausting resources. SHA-256 helpers provide
the integrity primitives required by capture and verification.

# Consequences

This creates a self-describing, authenticated envelope that storage can treat
as opaque. Tampering, an incorrect passphrase, or a purpose mismatch produces
an error and must halt the caller's restore pipeline. GPG signing and archive
container serialization are separate S9/S8 work items; neither is emulated by
this foundation.

# References

- `docs/01_Requirements.md`: NFR-1.1 through NFR-1.8, DR-2
- `docs/02_Architecture.md`: S9 — Crypto Engine
- `docs/03_Threat_Model.md`: TB-4, AS-4, AS-8
