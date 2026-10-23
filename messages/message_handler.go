package messages

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/spacelift-io/workercomms/crypto"
)

// PayloadHandler receives each message envelope a transport delivers to a worker.
type PayloadHandler interface {
	HandlePayload(workerULID string, payload []byte)
}

var _ PayloadHandler = (*MessageHandler)(nil)

// MessageHandlerFunc is used to handle a message received from Spacelift.
type MessageHandlerFunc func(messageHandler *MessageHandler, workerULID string, payload *ReceivedMessage)

// MessageHandlerOpts allows a MessageHandler to be configured.
type MessageHandlerOpts struct {
	MessageHandlers           map[MessageType]MessageHandlerFunc
	UnknownMessageTypeHandler func(MessageType)
}

type messageHandlerOptsFunc func(*MessageHandlerOpts)

// MessageHandler is used to handle messages sent from Spacelift to workers.
type MessageHandler struct {
	messageHandlers           map[MessageType]MessageHandlerFunc
	payloadErrorHandler       func(error)
	unknownMessageTypeHandler func(MessageType)
}

// NewMessageHandler creates a message handler for all the specified events. The behavior
// of the handler is slightly different depending on whether WithUnknownMessageTypeHandler() is used.
// If not specified, the handler will ignore any message types that it doesn't know how to handle,
// otherwise the specified callback will be called when an unknown message type is received.
func NewMessageHandler(payloadErrorHandler func(error), withOpts ...messageHandlerOptsFunc) *MessageHandler {
	opts := MessageHandlerOpts{
		MessageHandlers: make(map[MessageType]MessageHandlerFunc),
	}

	for _, withOpt := range withOpts {
		withOpt(&opts)
	}

	return &MessageHandler{
		messageHandlers:           opts.MessageHandlers,
		payloadErrorHandler:       payloadErrorHandler,
		unknownMessageTypeHandler: opts.UnknownMessageTypeHandler,
	}
}

// HandlePayload dispatches a message envelope received for workerULID.
func (h *MessageHandler) HandlePayload(workerULID string, payload []byte) {
	message, err := unmarshalMessage(payload)
	if err != nil {
		h.payloadError(err)
		return
	}

	h.dispatch(workerULID, message)
}

func (h *MessageHandler) dispatch(workerULID string, message *ReceivedMessage) {
	if handler, found := h.messageHandlers[message.MessageType]; found {
		handler(h, workerULID, message)
		return
	}

	if h.unknownMessageTypeHandler != nil {
		h.unknownMessageTypeHandler(message.MessageType)
	}
}

func unmarshalMessage(payload []byte) (*ReceivedMessage, error) {
	var message ReceivedMessage

	if err := json.Unmarshal(payload, &message); err != nil {
		return nil, errors.Wrapf(err, "invalid push event payload '%s'", payload)
	}

	return &message, nil
}

func panicIfHandlerAlreadyRegistered(opts *MessageHandlerOpts, messageType MessageType) {
	if _, exists := opts.MessageHandlers[messageType]; exists {
		panic(fmt.Sprintf("a message handler for message type '%s' already exists", messageType))
	}
}

func registerHandler(opts *MessageHandlerOpts, messageType MessageType, handler MessageHandlerFunc) {
	panicIfHandlerAlreadyRegistered(opts, messageType)

	opts.MessageHandlers[messageType] = handler
}

// WithJobAssignedHandler handles a job_assigned message. Unlike WithDecryptedJobAssignedHandler,
// it does not decrypt the message before passing it to the callback.
func WithJobAssignedHandler(handler func(workerULID string, data *crypto.EncryptedPayload)) messageHandlerOptsFunc {
	return func(opts *MessageHandlerOpts) {
		registerHandler(opts, MessageTypeJobAssigned, func(messageHandler *MessageHandler, workerULID string, payload *ReceivedMessage) {
			var data crypto.EncryptedPayload

			if err := json.Unmarshal(payload.MessagePayload, &data); err != nil {
				messageHandler.payloadError(errors.Wrapf(err, "invalid %s message payload '%s'", payload.MessageType, payload.MessagePayload))
				return
			}

			handler(workerULID, &data)
		})
	}
}

func (h *MessageHandler) payloadError(err error) {
	if h.payloadErrorHandler != nil {
		h.payloadErrorHandler(err)
	}
}

// WithDecryptedJobAssignedHandler handles a job_assigned message, and decrypts the message
// using the specified private key.
func WithDecryptedJobAssignedHandler(privateKey []byte, handler func(data []byte)) messageHandlerOptsFunc {
	return func(opts *MessageHandlerOpts) {
		registerHandler(opts, MessageTypeJobAssigned, func(messageHandler *MessageHandler, workerULID string, payload *ReceivedMessage) {
			var encryptedData crypto.EncryptedPayload

			if err := json.Unmarshal(payload.MessagePayload, &encryptedData); err != nil {
				messageHandler.payloadError(errors.Wrapf(err, "invalid %s message payload '%s'", payload.MessageType, payload.MessagePayload))
				return
			}

			decryptedData, err := crypto.DecryptPayload(privateKey, encryptedData)
			if err != nil {
				messageHandler.payloadError(err)
				return
			}

			handler(decryptedData)
		})
	}
}

type KillMessagePayload struct {
	Note string `json:"note"`
}

// WithKillHandler handles a kill message from Spacelift.
func WithKillHandler(handler func(workerULID string, payload *KillMessagePayload)) messageHandlerOptsFunc {
	return func(opts *MessageHandlerOpts) {
		registerHandler(opts, MessageTypeKill, func(messageHandler *MessageHandler, workerULID string, payload *ReceivedMessage) {
			var killPayload KillMessagePayload
			if err := json.Unmarshal(payload.MessagePayload, &killPayload); err != nil {
				messageHandler.payloadError(errors.Wrapf(err, "invalid %s message payload '%s'", payload.MessageType, payload.MessagePayload))
				return
			}

			handler(workerULID, &killPayload)
		})
	}
}

// WithStopHandler handles a stop message from Spacelift.
func WithStopHandler(handler func(payload json.RawMessage)) messageHandlerOptsFunc {
	return func(opts *MessageHandlerOpts) {
		registerHandler(opts, MessageTypeStop, func(messageHandler *MessageHandler, workerULID string, payload *ReceivedMessage) {
			handler(payload.MessagePayload)
		})
	}
}

// WithWorkloadSettledHandler handles a message saying that Spacelift no longer
// holds a workload for a worker that reports itself busy.
func WithWorkloadSettledHandler(handler func()) messageHandlerOptsFunc {
	return func(opts *MessageHandlerOpts) {
		registerHandler(opts, MessageTypeWorkloadSettled, func(*MessageHandler, string, *ReceivedMessage) {
			handler()
		})
	}
}

// WithScanAssignedHandler handles a scan_assigned message, and decrypts the message
// using the specified private key.
func WithScanAssignedHandler(privateKey []byte, handler func(data []byte)) messageHandlerOptsFunc {
	return func(opts *MessageHandlerOpts) {
		registerHandler(opts, MessageTypeScanAssigned, func(messageHandler *MessageHandler, workerULID string, payload *ReceivedMessage) {
			var encryptedData crypto.EncryptedPayload

			if err := json.Unmarshal(payload.MessagePayload, &encryptedData); err != nil {
				messageHandler.payloadError(errors.Wrapf(err, "invalid %s message payload '%s'", payload.MessageType, payload.MessagePayload))
				return
			}

			decryptedData, err := crypto.DecryptPayload(privateKey, encryptedData)
			if err != nil {
				messageHandler.payloadError(err)
				return
			}

			handler(decryptedData)
		})
	}
}

// WithIntentAssignedHandler handles an intent_assigned message, and decrypts the message
// using the specified private key.
func WithIntentAssignedHandler(privateKey []byte, handler func(data []byte)) messageHandlerOptsFunc {
	return func(opts *MessageHandlerOpts) {
		registerHandler(opts, MessageTypeIntentAssigned, func(messageHandler *MessageHandler, workerULID string, payload *ReceivedMessage) {
			var encryptedData crypto.EncryptedPayload

			if err := json.Unmarshal(payload.MessagePayload, &encryptedData); err != nil {
				messageHandler.payloadError(errors.Wrapf(err, "invalid %s message payload '%s'", payload.MessageType, payload.MessagePayload))
				return
			}

			decryptedData, err := crypto.DecryptPayload(privateKey, encryptedData)
			if err != nil {
				messageHandler.payloadError(err)
				return
			}

			handler(decryptedData)
		})
	}
}

// WithEncryptedIntentAssignedHandler handles an intent_assigned message without decrypting it.
// This is used by the kube-workerpool-controller which decrypts in the IntentManager
// after looking up the worker's private key.
func WithEncryptedIntentAssignedHandler(handler func(workerULID string, data *crypto.EncryptedPayload)) messageHandlerOptsFunc {
	return func(opts *MessageHandlerOpts) {
		registerHandler(opts, MessageTypeIntentAssigned, func(messageHandler *MessageHandler, workerULID string, payload *ReceivedMessage) {
			var data crypto.EncryptedPayload

			if err := json.Unmarshal(payload.MessagePayload, &data); err != nil {
				messageHandler.payloadError(errors.Wrapf(err, "invalid %s message payload '%s'", payload.MessageType, payload.MessagePayload))
				return
			}

			handler(workerULID, &data)
		})
	}
}

// WithEncryptedScanAssignedHandler handles an scan_assigned message without decrypting it.
// This is used by the kube-workerpool-controller which decrypts in the ScanManager
// after looking up the worker's private key.
func WithEncryptedScanAssignedHandler(handler func(workerULID string, data *crypto.EncryptedPayload)) messageHandlerOptsFunc {
	return func(opts *MessageHandlerOpts) {
		registerHandler(opts, MessageTypeScanAssigned, func(messageHandler *MessageHandler, workerULID string, payload *ReceivedMessage) {
			var data crypto.EncryptedPayload

			if err := json.Unmarshal(payload.MessagePayload, &data); err != nil {
				messageHandler.payloadError(errors.Wrapf(err, "invalid %s message payload '%s'", payload.MessageType, payload.MessagePayload))
				return
			}

			handler(workerULID, &data)
		})
	}
}

// WithUnknownMessageTypeHandler is used to opt-in to being notified when a message is received
// that there is no handler registered for.
func WithUnknownMessageTypeHandler(handler func(MessageType)) messageHandlerOptsFunc {
	return func(opts *MessageHandlerOpts) {
		if opts.UnknownMessageTypeHandler != nil {
			panic("unknown message type handler already registered")
		}

		opts.UnknownMessageTypeHandler = handler
	}
}
