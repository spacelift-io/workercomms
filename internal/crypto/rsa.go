package crypto

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/pem"
	"strings"

	"github.com/pkg/errors"
)

func parsePrivateKey(privateKey []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return nil, errors.New("could not decode PEM block containing private key")
	}

	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		if strings.Contains(err.Error(), "ParsePKCS8PrivateKey") {
			pkcs8Priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, errors.Wrap(err, "could not parse pkcs8 encoded private key")
			}
			rsaPriv, ok := pkcs8Priv.(*rsa.PrivateKey)
			if !ok {
				return nil, errors.Errorf("expected rsa private key, got %T", pkcs8Priv)
			}
			priv = rsaPriv
		} else {
			return nil, errors.Wrap(err, "could not parse pkcs1 encoded private key")
		}
	}

	return priv, nil
}

func parsePublicKey(publicKey []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(publicKey)
	if block == nil {
		return nil, errors.New("could not decode PEM block containing public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse encoded public key")
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.Errorf("unknown type of public key: %T", pub)
	}

	return rsaPub, nil
}

// GenerateRSAKeyPair generates a key pair suitable for encryption.
func GenerateRSAKeyPair() (public []byte, private []byte, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not generate RSA key")
	}

	pubASN1, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not marshal PKIX public key")
	}

	privASN1, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldn't pkcs8 marshal private key")
	}

	var pubBuffer bytes.Buffer
	if err := pem.Encode(&pubBuffer, &pem.Block{Type: "RSA PUBLIC KEY", Bytes: pubASN1}); err != nil {
		return nil, nil, errors.Wrap(err, "couldn't pem-encode launcher public key")
	}

	var privBuffer bytes.Buffer
	if err := pem.Encode(&privBuffer, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privASN1}); err != nil {
		return nil, nil, errors.Wrap(err, "couldn't pem-encode launcher private key")
	}

	return pubBuffer.Bytes(), privBuffer.Bytes(), nil
}

// EncryptRSA encrypts message using public key (x509 format).
func EncryptRSA(publicKey, msg []byte) ([]byte, error) {
	rsaPub, err := parsePublicKey(publicKey)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't parse public key")
	}

	return rsa.EncryptOAEP(sha512.New(), rand.Reader, rsaPub, msg, nil)
}

// DecryptRSA decrypts ciphertext using private key (x509 format).
func DecryptRSA(privateKey, ciphertext []byte) ([]byte, error) {
	priv, err := parsePrivateKey(privateKey)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't parse private key")
	}

	return rsa.DecryptOAEP(sha512.New(), rand.Reader, priv, ciphertext, nil)
}
