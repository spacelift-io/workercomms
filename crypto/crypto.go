package crypto

import "github.com/spacelift-io/workercomms/internal/crypto"

func GenerateRSAKeyPair() (public []byte, private []byte, err error) {
	return crypto.GenerateRSAKeyPair()
}
