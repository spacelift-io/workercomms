package messages

import "encoding/json"

// MessageType defines a type of message exchanged between Spacelift and workers.
type MessageType string

const (
	// MessageTypeHeartbeat represents "heartbeat" payload type.
	MessageTypeHeartbeat MessageType = "heartbeat"

	// MessageTypeJobAssigned represents "job_assigned" payload type.
	MessageTypeJobAssigned MessageType = "job_assigned"

	// MessageTypeKill represents "kill" payload type.
	MessageTypeKill MessageType = "kill"

	// MessageTypeStop represents "stop" payload type.
	MessageTypeStop MessageType = "stop"

	// MessageTypeWorkerGone represents "worker_gone" payload type.
	MessageTypeWorkerGone MessageType = "worker_gone"

	// MessageTypeWorkerReady represents "worker_ready" payload type.
	MessageTypeWorkerReady MessageType = "worker_ready"

	// MessageTypeRunComplete represents "run_complete" payload type.
	MessageTypeRunComplete MessageType = "run_complete"

	// MessageTypeIntentAssigned represents "intent_assigned" payload type.
	MessageTypeIntentAssigned MessageType = "intent_assigned"

	// MessageTypeScanAssigned represents "scan_assigned" payload type.
	MessageTypeScanAssigned MessageType = "scan_assigned"

	// MessageTypeWorkloadSettled represents "workload_settled" payload type.
	MessageTypeWorkloadSettled MessageType = "workload_settled"
)

// Message is the envelope of a message sent to or from a worker.
type Message struct {
	MessageType    MessageType `json:"message_type"`
	MessagePayload any         `json:"message_payload"`
}

// ReceivedMessage is a Message as received, with its payload left raw.
type ReceivedMessage struct {
	MessageType    MessageType     `json:"message_type"`
	MessagePayload json.RawMessage `json:"message_payload"`
}
