package crypto

import (
	"bytes"

	"github.com/pkg/errors"
)

// pkcs7Unpad provides PKCS #7 unpadding.
// https://tools.ietf.org/html/rfc2315#section-10.3
func pkcs7Unpad(msg []byte, blockSize int) ([]byte, error) {
	if blockSize <= 0 {
		return nil, errors.Errorf("invalid block size (%d)", blockSize)
	}

	if len(msg) == 0 {
		return nil, errors.New("empty message")
	}

	if len(msg)%blockSize != 0 {
		return nil, errors.New("message is not a multiple of the block size")
	}

	// A corrupt message or a wrong key leaves an arbitrary last byte, which
	// must be rejected rather than trusted as a length.
	padding := int(msg[len(msg)-1])
	if padding == 0 || padding > blockSize {
		return nil, errors.Errorf("invalid padding length (%d)", padding)
	}

	for _, b := range msg[len(msg)-padding:] {
		if int(b) != padding {
			return nil, errors.New("invalid padding bytes")
		}
	}

	return msg[:len(msg)-padding], nil
}

// pkcs7Pad provides PKCS#7 padding mechanism.
// https://tools.ietf.org/html/rfc2315#section-10.3
func pkcs7Pad(msg []byte, blockSize int) ([]byte, error) {
	if blockSize <= 0 {
		return nil, errors.Errorf("invalid block size (%d)", blockSize)
	}

	padding := blockSize - (len(msg) % blockSize)
	paddedMsg := bytes.Repeat([]byte{byte(padding)}, padding) // #nosec G115 -- PKCS#7 padding is at most one block

	return append(msg, paddedMsg...), nil
}
