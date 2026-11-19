package mqtt

import (
	"time"

	"github.com/spacelift-io/workercomms/capability"
	"github.com/spacelift-io/workercomms/messages"
)

// NewHeartBeatMessage creates a new heartbeat for the specified timestamp.
func NewHeartBeatMessage(t time.Time) messages.Message {
	return messages.Message{
		MessageType:    messages.MessageTypeHeartbeat,
		MessagePayload: t,
	}
}

type WorkerReadyPayload struct {
	PublicKey      string                  `json:"public_key"`
	BuildTimestamp *int64                  `json:"build_timestamp,omitempty"`
	Metadata       map[string]any          `json:"metadata,omitempty"`
	Capabilities   []capability.Capability `json:"capabilities,omitempty"`
}

// NewWorkerReadyMessage creates a new worker_ready message.
func NewWorkerReadyMessage(publicKey string, buildTimestamp int64, metadata map[string]any, capabilities []capability.Capability) messages.Message {
	return messages.Message{
		MessageType: messages.MessageTypeWorkerReady,
		MessagePayload: WorkerReadyPayload{
			PublicKey:      publicKey,
			BuildTimestamp: &buildTimestamp,
			Metadata:       metadata,
			Capabilities:   capabilities,
		},
	}
}

// NewWorkerGoneMessage creates a new worker_gone message.
func NewWorkerGoneMessage(crashed bool) messages.Message {
	payload := map[string]interface{}{}
	if crashed {
		payload["crashed"] = true
	}

	return messages.Message{
		MessageType:    messages.MessageTypeWorkerGone,
		MessagePayload: payload,
	}
}

// NewRunCompleteMessage creates a new RunComplete message. This message is used to indicate
// that the run has completed from the worker's perspective. This is used as the last will
// for Kubernetes workers as a failsafe to prevent a situation where the worker crashes in
// such a way that neither the run pod or the controller manage to notify the Spacelift backend
// of success or failure.
func NewRunCompleteMessage(runULID string) messages.Message {
	payload := map[string]interface{}{
		"run_ulid": runULID,
	}

	return messages.Message{
		MessageType:    messages.MessageTypeRunComplete,
		MessagePayload: payload,
	}
}
