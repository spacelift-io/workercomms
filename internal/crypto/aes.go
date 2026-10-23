package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"

	"github.com/pkg/errors"
)

// EncryptAES encrypts message using AES-256-CBC algorithm
func EncryptAES(key, msg []byte) ([]byte, error) {
	// AES uses 16 bytes block size.
	paddedMsg, err := pkcs7Pad(msg, aes.BlockSize)
	if err != nil {
		return nil, errors.Wrap(err, "could not pad the message")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err, "could not create an AES cipher")
	}

	ciphertext := make([]byte, aes.BlockSize+len(paddedMsg))

	// Generating initialization vector
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, errors.Wrap(err, "could not generate an initialization vector")
	}

	cbc := cipher.NewCBCEncrypter(block, iv) //nolint:gosec // G407 false positive: iv is filled from crypto/rand above
	cbc.CryptBlocks(ciphertext[aes.BlockSize:], paddedMsg)

	return ciphertext, nil
}

// DecryptAES decrypts ciphertext in AES-256-CBC.
func DecryptAES(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext is too short")
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err, "could not create an AES cipher")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	cbc := cipher.NewCBCDecrypter(block, iv)
	cbc.CryptBlocks(ciphertext, ciphertext)

	plaintext, err := pkcs7Unpad(ciphertext, aes.BlockSize)
	if err != nil {
		return nil, errors.Wrap(err, "could not unpad the message")
	}

	return plaintext, nil
}
