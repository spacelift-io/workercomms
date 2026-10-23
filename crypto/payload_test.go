package crypto_test

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spacelift-io/workercomms/crypto"
)

var keyPair = sync.OnceValues(func() ([2][]byte, error) {
	public, private, err := crypto.GenerateRSAKeyPair()
	return [2][]byte{public, private}, err
})

func TestDecryptPayload(t *testing.T) {
	keys, err := keyPair()
	require.NoError(t, err)
	public, private := keys[0], keys[1]

	_, otherPrivate, err := crypto.GenerateRSAKeyPair()
	require.NoError(t, err)

	payload := []byte(`{"id":"abc123"}`)
	valid := encrypt(t, public, payload)

	cases := []struct {
		name        string
		privateKey  []byte
		msg         crypto.EncryptedPayload
		expectedErr string
	}{
		{
			name:       "valid payload",
			privateKey: private,
			msg:        valid,
		},
		{
			name:        "data key not base64",
			privateKey:  private,
			msg:         crypto.EncryptedPayload{EncryptedDataKey: "!!!", EncryptedRequest: valid.EncryptedRequest},
			expectedErr: "could not decode base64 key",
		},
		{
			name:        "request not base64",
			privateKey:  private,
			msg:         crypto.EncryptedPayload{EncryptedDataKey: valid.EncryptedDataKey, EncryptedRequest: "!!!"},
			expectedErr: "could not decode base64 workspace data",
		},
		{
			name:        "invalid private key",
			privateKey:  []byte("Fake key!!!"),
			msg:         valid,
			expectedErr: "could not decode encrypted key",
		},
		{
			name:        "wrong private key",
			privateKey:  otherPrivate,
			msg:         valid,
			expectedErr: "could not decode encrypted key",
		},
		{
			name:       "corrupted request",
			privateKey: private,
			msg: crypto.EncryptedPayload{
				EncryptedDataKey: valid.EncryptedDataKey,
				EncryptedRequest: base64.StdEncoding.EncodeToString([]byte("Fake encrypted payload")),
			},
			expectedErr: "could not decode encrypted workspace data",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			decrypted, err := crypto.DecryptPayload(c.privateKey, c.msg)
			if c.expectedErr != "" {
				assert.ErrorContains(t, err, c.expectedErr)
				assert.Nil(t, decrypted)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, payload, decrypted)
		})
	}
}

func encrypt(t *testing.T, publicKeyPEM, payload []byte) crypto.EncryptedPayload {
	t.Helper()

	dataKey := make([]byte, 32)
	_, err := rand.Read(dataKey)
	require.NoError(t, err)

	encrypted, err := crypto.EncryptPayload(dataKey, publicKeyPEM, payload)
	require.NoError(t, err)

	return *encrypted
}
