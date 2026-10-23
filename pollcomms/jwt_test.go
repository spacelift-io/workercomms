package pollcomms_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spacelift-io/workercomms/pollcomms"
)

// poolKeyPair returns the PEM shapes the pool certificate holds: a PKCS1
// private key and a PKIX public key.
func poolKeyPair(t *testing.T) (key *rsa.PrivateKey, privatePEM, publicPEM []byte) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privatePEM = pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	publicPEM = pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})

	return key, privatePEM, publicPEM
}

func TestSign(t *testing.T) {
	key, privatePEM, _ := poolKeyPair(t)

	token, err := pollcomms.Sign(privatePEM, "pool-ulid", "worker-ulid")
	require.NoError(t, err)

	claims := &pollcomms.Claims{}
	_, err = jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return &key.PublicKey, nil
	}, jwt.WithValidMethods([]string{"RS256"}))
	require.NoError(t, err)

	assert.Equal(t, "pool-ulid", claims.PoolULID)
	assert.Equal(t, "worker-ulid", claims.WorkerULID)
	assert.Equal(t, "worker-ulid", claims.Subject)
	assert.Equal(t, pollcomms.TokenTTL, claims.ExpiresAt.Sub(claims.IssuedAt.Time))
	assert.WithinDuration(t, time.Now(), claims.IssuedAt.Time, time.Minute)
}

func TestSignRejectsInvalidKey(t *testing.T) {
	_, err := pollcomms.Sign([]byte("not a key"), "pool-ulid", "worker-ulid")
	assert.ErrorContains(t, err, "could not parse pool private key")
}

func TestSignVerifyRoundtrip(t *testing.T) {
	_, privatePEM, publicPEM := poolKeyPair(t)

	token, err := pollcomms.Sign(privatePEM, "pool-ulid", "worker-ulid")
	require.NoError(t, err)

	unverified, err := pollcomms.UnverifiedClaims(token)
	require.NoError(t, err)
	assert.Equal(t, "pool-ulid", unverified.PoolULID)

	claims, err := pollcomms.Verify(token, publicPEM)
	require.NoError(t, err)
	assert.Equal(t, "worker-ulid", claims.WorkerULID)
	assert.Equal(t, "pool-ulid", claims.PoolULID)
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	_, privatePEM, _ := poolKeyPair(t)
	_, _, otherPublicPEM := poolKeyPair(t)

	token, err := pollcomms.Sign(privatePEM, "pool-ulid", "worker-ulid")
	require.NoError(t, err)

	_, err = pollcomms.Verify(token, otherPublicPEM)
	assert.ErrorContains(t, err, "could not verify pool token")
}

func TestVerifyRejectsNonRS256(t *testing.T) {
	_, _, publicPEM := poolKeyPair(t)

	// HS256 signed with the public key bytes is the algorithm-confusion attack.
	hsToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, pollcomms.Claims{
		WorkerULID: "worker-ulid",
		PoolULID:   "pool-ulid",
	}).SignedString(publicPEM)
	require.NoError(t, err)

	_, err = pollcomms.Verify(hsToken, publicPEM)
	assert.ErrorContains(t, err, "could not verify pool token")
}

func TestVerifyRejectsInvalidKey(t *testing.T) {
	_, privatePEM, _ := poolKeyPair(t)

	token, err := pollcomms.Sign(privatePEM, "pool-ulid", "worker-ulid")
	require.NoError(t, err)

	_, err = pollcomms.Verify(token, []byte("not a key"))
	assert.ErrorContains(t, err, "could not parse pool public key")
}

func TestUnverifiedClaimsRejectsMissingClaims(t *testing.T) {
	_, privatePEM, _ := poolKeyPair(t)

	token, err := pollcomms.Sign(privatePEM, "", "")
	require.NoError(t, err)

	_, err = pollcomms.UnverifiedClaims(token)
	assert.ErrorContains(t, err, "token missing pool or worker claim")
}

func TestUnverifiedClaimsRejectsGarbage(t *testing.T) {
	_, err := pollcomms.UnverifiedClaims("not a token")
	assert.ErrorContains(t, err, "could not parse pool token")
}
