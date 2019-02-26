package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"

	secp256k1 "github.com/ipsn/go-secp256k1"
)

// PrivateKeyBytes is the size of a serialized private key.
const PrivateKeyBytes = 32

// PublicKeyBytes is the size of a serialized public key.
const PublicKeyBytes = 65

// PrivateKey is the in memory representation of a secp256k1 Private Key.
type PrivateKey [PrivateKeyBytes]byte

// PublicKey is the in memory representation of a secp256k1 Public Key.
type PublicKey [PublicKeyBytes]byte

// PublicKey returns the public key for this private key.
func (pk *PrivateKey) PublicKey() *PublicKey {
	x, y := secp256k1.S256().ScalarBaseMult(pk[:])
	pubkeyBytes := elliptic.Marshal(secp256k1.S256(), x, y)

	pubkey := PublicKey{}
	copy(pubkey[:], pubkeyBytes[:])

	return &pubkey
}

// Sign signs the given message, which must be 32 bytes long.
func (pk *PrivateKey) Sign(msg []byte) ([]byte, error) {
	return secp256k1.Sign(msg, pk[:])
}

// Equals compares two private key for equality and returns true if they are the same.
func (pk *PrivateKey) Equals(other *PrivateKey) bool {
	return bytes.Equal(pk[:], other[:])
}

// Serialize returns the byte representation of the private key.
func (pk *PrivateKey) Serialize() []byte {
	return pk[:]
}

// MarshalJSON marshals the private key to json and returns the bytes.
func (pk *PrivateKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(pk[:])
}

// UnmarshalJSON unmarshals the byte array to a private key.
func (pk *PrivateKey) UnmarshalJSON(b []byte) error {
	bytes := make([]byte, PrivateKeyBytes)
	err := json.Unmarshal(b, &bytes)
	if err != nil {
		return err
	}

	copy(pk[:], bytes[:])

	return nil
}

// Serialize returns the byte representation of the public key.
func (pk *PublicKey) Serialize() []byte {
	return pk[:]
}

// Verify checks the given signature and returns true if it is valid.
func (pk *PublicKey) Verify(msg, signature []byte) bool {
	if len(signature) == 65 {
		// Drop the V in [R | S | V] style signatures
		return secp256k1.VerifySignature(pk[:], msg, signature[:len(signature)-2])
	}

	return secp256k1.VerifySignature(pk[:], msg, signature)
}

// GenerateKeyFromSeed generates a new key from the given reader.
func GenerateKeyFromSeed(seed io.Reader) (*PrivateKey, error) {
	key, err := ecdsa.GenerateKey(secp256k1.S256(), seed)
	if err != nil {
		return nil, err
	}

	privkey := PrivateKey{}
	blob := key.D.Bytes()

	copy(privkey[PrivateKeyBytes-len(blob):], blob)

	return &privkey, nil
}

// GenerateKey creates a new key using secure randomness from crypto.rand.
func GenerateKey() (*PrivateKey, error) {
	return GenerateKeyFromSeed(rand.Reader)
}

// EcRecover recovers the public key from a message, signature pair.
func EcRecover(msg, signature []byte) (*PublicKey, error) {
	key, err := secp256k1.RecoverPubkey(msg, signature)
	if err != nil {
		return nil, err
	}

	pubkey := PublicKey{}
	copy(pubkey[:], key[:])
	return &pubkey, nil
}
