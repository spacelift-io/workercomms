package pollcomms

// Wire constants for the worker poll transport.
const (
	APITokenHeader = "Authorization"

	URLPoll   = "/worker/poll"
	URLAck    = "/worker/ack/{messageULID}"
	URLGone   = "/worker/gone"
	URLRetire = "/worker/retire"
)
