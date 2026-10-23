package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateRSAKeyPair(t *testing.T) {
	publicKeyBytes, privateKeyBytes, err := GenerateRSAKeyPair()
	require.NoError(t, err)

	privKey, err := parsePrivateKey(privateKeyBytes)
	require.NoError(t, err)
	assert.NoError(t, privKey.Validate())
	assert.Equal(t, 4096, privKey.N.BitLen())

	pubKey, err := parsePublicKey(publicKeyBytes)
	require.NoError(t, err)
	assert.True(t, privKey.PublicKey.Equal(pubKey))
}

func TestEncryptRSA_Errors(t *testing.T) {
	cases := []struct {
		name        string
		pubKey      []byte
		msg         []byte
		expectedErr string
	}{
		{
			name:        "invalid PEM block",
			pubKey:      []byte("----BEGIN"),
			expectedErr: "couldn't parse public key: could not decode PEM block containing public key",
		},
		{
			name:        "invalid public key",
			pubKey:      pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: []byte("becon")}),
			expectedErr: "could not parse encoded public key: asn1",
		},
		{
			name: "non-RSA public key",
			pubKey: func() []byte {
				public, _, err := ed25519.GenerateKey(rand.Reader)
				require.NoError(t, err)
				der, err := x509.MarshalPKIXPublicKey(public)
				require.NoError(t, err)
				return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
			}(),
			expectedErr: "unknown type of public key: ed25519.PublicKey",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ciphertext, err := EncryptRSA(c.pubKey, c.msg)
			assert.Nil(t, ciphertext)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), c.expectedErr)
		})
	}
}

func TestDecryptRSA_Errors(t *testing.T) {
	cases := []struct {
		name        string
		privKey     []byte
		msg         []byte
		expectedErr string
	}{
		{
			name:        "invalid PEM block",
			privKey:     []byte("----BEGIN"),
			expectedErr: "couldn't parse private key: could not decode PEM block containing private key",
		},
		{
			name:        "invalid private key",
			privKey:     pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("becon")}),
			expectedErr: "could not parse pkcs1 encoded private key: asn1",
		},
		{
			name: "empty cipher text",
			privKey: func() []byte {
				_, privKey, _ := GenerateRSAKeyPair()
				return privKey
			}(),
			expectedErr: "crypto/rsa: decryption error",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ciphertext, err := DecryptRSA(c.privKey, c.msg)
			assert.Nil(t, ciphertext)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), c.expectedErr)
		})
	}
}

func TestEncryptDecryptRSA(t *testing.T) {
	publicKey, privateKey, err := GenerateRSAKeyPair()
	require.NoError(t, err)

	msg := []byte("alice is talking to bob")
	cipherText, err := EncryptRSA(publicKey, msg)
	require.NoError(t, err)

	clearText, err := DecryptRSA(privateKey, cipherText)
	require.NoError(t, err)
	assert.Equal(t, msg, clearText)
}
