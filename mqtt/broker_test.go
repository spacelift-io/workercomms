package mqtt

import (
	"fmt"
	"io"
	"net"
	"os"
	"testing"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

// connectFailure rebuilds what connectWithRetry returns: paho joins its own
// sentinel to the underlying cause with fmt.Errorf("%w : %w", ...).
func connectFailure(cause error) error {
	return errors.Wrapf(
		fmt.Errorf("%w : %w", packets.ErrorNetworkError, cause),
		"could not connect to MQTT broker after %d retries", 4,
	)
}

func TestIsNetworkError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "handshake dropped before the CONNACK",
			err:  connectFailure(io.EOF),
			want: true,
		},
		{
			name: "dial never landed",
			err:  connectFailure(&net.OpError{Op: "dial", Err: os.ErrDeadlineExceeded}),
			want: true,
		},
		{
			name: "bare net error without paho's sentinel",
			err:  errors.Wrap(&net.OpError{Op: "dial", Err: os.ErrDeadlineExceeded}, "could not connect to MQTT broker"),
			want: true,
		},
		{
			name: "broker refused the certificate",
			err:  errors.Wrap(packets.ErrorRefusedNotAuthorised, "could not connect to MQTT broker after 4 retries"),
			want: false,
		},
		{
			name: "broker rejected the client id",
			err:  errors.Wrap(packets.ErrorRefusedIDRejected, "could not connect to MQTT broker after 4 retries"),
			want: false,
		},
		{
			name: "subscribing failed for an unrelated reason",
			err:  errors.New("couldn't subscribe to messages"),
			want: false,
		},
		{
			name: "no error",
			err:  nil,
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsNetworkError(tc.err))
		})
	}
}

// TestIsNetworkErrorCoversWhatNetErrorMisses pins the reason a net.Error type
// assertion alone let the handshake failure through: paho reports it as its own
// sentinel joined to io.EOF, and neither of those is a net.Error.
func TestIsNetworkErrorCoversWhatNetErrorMisses(t *testing.T) {
	err := connectFailure(io.EOF)

	var netErr net.Error
	assert.False(t, errors.As(err, &netErr))

	assert.True(t, IsNetworkError(err))
}
