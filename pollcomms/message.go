package pollcomms

import "time"

// MaxPollHold stays under the proxy/LB idle timeout (ALB default 60s) so idle
// polls end in a server 204, not a proxy-induced 504 + client backoff.
const MaxPollHold = 55 * time.Second

// PollRequest is the body a worker sends with every poll.
type PollRequest struct {
	PublicKey      string         `json:"public_key"`
	BuildTimestamp *int64         `json:"build_timestamp,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Available      *bool          `json:"available,omitempty"`
}

// IsAvailable reports whether the worker can take work; workers that predate Available always can.
func (r PollRequest) IsAvailable() bool {
	return r.Available == nil || *r.Available
}

// Ack is by ULID; the message type lives inside Payload (a messages.Message).
type PollResponse struct {
	ULID    string `json:"ulid"`
	Payload []byte `json:"payload"`
}
