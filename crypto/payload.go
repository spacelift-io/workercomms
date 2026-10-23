package crypto

import (
	"encoding/base64"

	"github.com/pkg/errors"

	"github.com/spacelift-io/workercomms/internal/crypto"
)

// EncryptedPayload represents the encrypted run workspace information.
type EncryptedPayload struct {
	EncryptedDataKey string `json:"encrypted_data_key"`
	EncryptedRequest string `json:"encrypted_request"`
}

func EncryptPayload(dataKey []byte, publicKey []byte, payload []byte) (*EncryptedPayload, error) {
	encData, err := crypto.EncryptAES(dataKey, payload)
	if err != nil {
		return nil, err
	}
	encKey, err := crypto.EncryptRSA(publicKey, dataKey)
	if err != nil {
		return nil, err
	}
	return &EncryptedPayload{
		EncryptedDataKey: base64.StdEncoding.EncodeToString(encKey),
		EncryptedRequest: base64.StdEncoding.EncodeToString(encData),
	}, nil
}

// DecryptPayload is used to decrypt a message encrypted using public key encryption.
func DecryptPayload(privateKey []byte, msg EncryptedPayload) ([]byte, error) {
	encKey, err := base64.StdEncoding.DecodeString(msg.EncryptedDataKey)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode base64 key")
	}

	encData, err := base64.StdEncoding.DecodeString(msg.EncryptedRequest)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode base64 workspace data")
	}

	aesKey, err := crypto.DecryptRSA(privateKey, encKey)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode encrypted key")
	}

	jsonData, err := crypto.DecryptAES(aesKey, encData)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode encrypted workspace data")
	}

	return jsonData, nil
}
