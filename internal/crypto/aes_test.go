package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecryptAES_Errors(t *testing.T) {
	cases := []struct {
		name        string
		key         []byte
		cipherText  []byte
		expectedErr string
	}{
		{
			name:        "ciphertext is too short",
			cipherText:  []byte("FJN6D"),
			expectedErr: "ciphertext is too short",
		},
		{
			name:        "ciphertext is not a multiple of the block size",
			cipherText:  []byte("DSGJDGB84BSDKJA4SDAURAS83BASD93"), // 31 bytes
			expectedErr: "ciphertext is not a multiple of the block size",
		},
		{
			name:        "empty key",
			cipherText:  []byte("DSGJDGB84BSDKJAE"),
			expectedErr: "could not create an AES cipher: crypto/aes: invalid key size 0",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clearText, err := DecryptAES(c.key, c.cipherText)
			assert.Nil(t, clearText)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), c.expectedErr)
		})
	}
}

func TestEncryptDecryptAES(t *testing.T) {
	key := []byte("1111111111111111") // 16 bytes
	msg := []byte("alice is talking to bob")
	cipherText, err := EncryptAES(key, msg)
	require.NoError(t, err)

	clearText, err := DecryptAES(key, cipherText)
	require.NoError(t, err)
	assert.Equal(t, msg, clearText)
}
