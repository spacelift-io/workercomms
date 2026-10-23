package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPKCS7Unpad(t *testing.T) {
	block := func(tail ...byte) []byte {
		return append(bytes.Repeat([]byte{'a'}, 16-len(tail)), tail...)
	}

	cases := []struct {
		name        string
		msg         []byte
		expected    []byte
		expectedErr string
	}{
		{
			name:     "strips one padding byte",
			msg:      block(1),
			expected: bytes.Repeat([]byte{'a'}, 15),
		},
		{
			name:     "strips a whole padding block",
			msg:      append(bytes.Repeat([]byte{'a'}, 16), bytes.Repeat([]byte{16}, 16)...),
			expected: bytes.Repeat([]byte{'a'}, 16),
		},
		{
			name:        "zero padding length",
			msg:         block(0),
			expectedErr: "invalid padding length (0)",
		},
		{
			name:        "padding longer than the block",
			msg:         block(200),
			expectedErr: "invalid padding length (200)",
		},
		{
			name:        "padding bytes that disagree",
			msg:         block(2, 3, 3),
			expectedErr: "invalid padding bytes",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plaintext, err := pkcs7Unpad(c.msg, 16)
			if c.expectedErr != "" {
				assert.Nil(t, plaintext)
				assert.EqualError(t, err, c.expectedErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.expected, plaintext)
		})
	}
}
