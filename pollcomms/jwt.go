package pollcomms

import (
	"crypto/rsa"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
)

const TokenTTL = 5 * time.Minute

type Claims struct {
	jwt.RegisteredClaims
	WorkerULID string `json:"wrk"`
	PoolULID   string `json:"pool"`
}

func Sign(privateKeyPEM []byte, poolULID, workerULID string) (string, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return "", errors.Wrap(err, "could not parse pool private key")
	}

	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   workerULID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
		},
		WorkerULID: workerULID,
		PoolULID:   poolULID,
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	return signed, errors.Wrap(err, "could not sign pool token")
}

// ParsePublicKey loads a pool public key the way Verify does, so a caller can reject a certificate no token could verify against.
func ParsePublicKey(publicKeyPEM []byte) (*rsa.PublicKey, error) {
	key, err := jwt.ParseRSAPublicKeyFromPEM(publicKeyPEM)
	return key, errors.Wrap(err, "could not parse pool public key")
}

// UnverifiedClaims reads the claims before the signature is checked, so the
// caller can pick the pool key for Verify. A forged pool ULID still can't sign
// against that pool's real key.
func UnverifiedClaims(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	if _, _, err := jwt.NewParser().ParseUnverified(tokenStr, claims); err != nil {
		return nil, errors.Wrap(err, "could not parse pool token")
	}
	if claims.PoolULID == "" || claims.WorkerULID == "" {
		return nil, errors.New("token missing pool or worker claim")
	}
	return claims, nil
}

// Verify enforces RS256 only (algorithm-confusion guard) and checks the
// signature against the pool certificate. There is deliberately no audience
// check: the per-pool key signs and verifies tokens for this transport alone, so
// a valid signature already proves pool-key possession, and with a single shared
// base URL an audience claim discriminates nothing.
func Verify(tokenStr string, publicKeyPEM []byte) (*Claims, error) {
	key, err := ParsePublicKey(publicKeyPEM)
	if err != nil {
		return nil, err
	}

	claims := &Claims{}
	if _, err := jwt.ParseWithClaims(tokenStr, claims, func(*jwt.Token) (any, error) {
		return key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()})); err != nil {
		return nil, errors.Wrap(err, "could not verify pool token")
	}
	return claims, nil
}
