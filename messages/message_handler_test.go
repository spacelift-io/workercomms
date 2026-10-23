package messages_test

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/spacelift-io/workercomms/crypto"
	internalcrypto "github.com/spacelift-io/workercomms/internal/crypto"
	"github.com/spacelift-io/workercomms/messages"
)

var invalidJSONPayload = []byte(`{"key":"va`)

type MessageHandlerTests struct {
	suite.Suite

	messageHandler *messages.MessageHandler
	opts           messages.MessageHandlerOpts
	payloadError   error
}

func (d *MessageHandlerTests) SetupTest() {
	d.payloadError = nil
	d.messageHandler = messages.NewMessageHandler(func(err error) {
		d.payloadError = err
	})
	d.opts = messages.MessageHandlerOpts{
		MessageHandlers: make(map[messages.MessageType]messages.MessageHandlerFunc),
	}
}

func (t *MessageHandlerTests) TestWithJobAssignedHandler_SetsHandler() {
	optsFunc := messages.WithJobAssignedHandler(func(workerULID string, data *crypto.EncryptedPayload) {})

	optsFunc(&t.opts)

	t.NotNil(t.opts.MessageHandlers[messages.MessageTypeJobAssigned])
}

func (t *MessageHandlerTests) TestWithJobAssignedHandler_RegisteredTwice_Panics() {
	optsFunc := messages.WithJobAssignedHandler(func(workerULID string, data *crypto.EncryptedPayload) {})

	optsFunc(&t.opts)
	t.Panics(func() { optsFunc(&t.opts) })
}

func (t *MessageHandlerTests) TestWithJobAssignedHandler_PayloadValid_UnmarshalsIotPayload() {
	var data *crypto.EncryptedPayload
	optsFunc := messages.WithJobAssignedHandler(func(workerULID string, d *crypto.EncryptedPayload) {
		data = d
	})
	optsFunc(&t.opts)
	payload := messages.ReceivedMessage{
		MessageType:    messages.MessageTypeJobAssigned,
		MessagePayload: json.RawMessage(`{"encrypted_data_key":"key","encrypted_request":"request"}`),
	}

	t.opts.MessageHandlers[messages.MessageTypeJobAssigned](t.messageHandler, "", &payload)

	t.NoError(t.payloadError)
	t.NotNil(data)
	t.Equal("key", data.EncryptedDataKey)
	t.Equal("request", data.EncryptedRequest)
}

func (t *MessageHandlerTests) TestWithJobAssignedHandler_PayloadInvalid_CallsErrorHandler() {
	var data *crypto.EncryptedPayload
	optsFunc := messages.WithJobAssignedHandler(func(workerULID string, d *crypto.EncryptedPayload) {
		data = d
	})
	optsFunc(&t.opts)
	payload := messages.ReceivedMessage{
		MessageType:    messages.MessageTypeJobAssigned,
		MessagePayload: json.RawMessage(invalidJSONPayload),
	}

	t.opts.MessageHandlers[messages.MessageTypeJobAssigned](t.messageHandler, "", &payload)

	t.Nil(data)
	t.ErrorContains(t.payloadError, fmt.Sprintf("invalid job_assigned message payload '%s'", invalidJSONPayload))
}

func (t *MessageHandlerTests) TestWithDecryptedJobAssignedHandler_SetsHandler() {
	optsFunc := messages.WithDecryptedJobAssignedHandler([]byte("abc123"), func(data []byte) {})

	optsFunc(&t.opts)

	t.NotNil(t.opts.MessageHandlers[messages.MessageTypeJobAssigned])
}

func (t *MessageHandlerTests) TestWithDecryptedJobAssignedHandler_RegisteredTwice_Panics() {
	optsFunc := messages.WithDecryptedJobAssignedHandler([]byte("abc123"), func(data []byte) {})

	optsFunc(&t.opts)
	t.Panics(func() { optsFunc(&t.opts) })
}

func (t *MessageHandlerTests) generateEncryptedRequest(payload string) (privateKey []byte, jsonPayload []byte, encryptedKey []byte) {
	aesKey := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, aesKey)
	t.NoError(err)

	launcherPublicKey, launcherPrivateKey, err := internalcrypto.GenerateRSAKeyPair()
	t.NoError(err)

	encryptedJSON, err := internalcrypto.EncryptAES(aesKey, []byte(payload))
	t.NoError(err)

	encryptedKey, err = internalcrypto.EncryptRSA(launcherPublicKey, aesKey)
	t.NoError(err)

	return launcherPrivateKey, encryptedJSON, encryptedKey
}

func (t *MessageHandlerTests) TestWithDecryptedJobAssignedHandler_PayloadValid_DecryptsPayload() {
	launcherPrivateKey, encryptedJSON, encryptedKey := t.generateEncryptedRequest(`{id:"abc123"}`)

	encryptedWorkspaceData := crypto.EncryptedPayload{
		EncryptedDataKey: base64.StdEncoding.EncodeToString(encryptedKey),
		EncryptedRequest: base64.StdEncoding.EncodeToString(encryptedJSON),
	}

	encryptedWorkspaceDataMarshalled, err := json.Marshal(&encryptedWorkspaceData)
	t.NoError(err)

	var decryptedData []byte
	optsFunc := messages.WithDecryptedJobAssignedHandler(launcherPrivateKey, func(d []byte) {
		decryptedData = d
	})
	optsFunc(&t.opts)
	payload := messages.ReceivedMessage{
		MessageType:    messages.MessageTypeJobAssigned,
		MessagePayload: json.RawMessage(encryptedWorkspaceDataMarshalled),
	}

	t.opts.MessageHandlers[messages.MessageTypeJobAssigned](t.messageHandler, "", &payload)

	t.NoError(t.payloadError)
	t.Equal(`{id:"abc123"}`, string(decryptedData))
}

func (t *MessageHandlerTests) TestWithDecryptedJobAssignedHandler_EncryptionKeyNotEncodedCorrectly_CallsErrorHandler() {
	launcherPrivateKey, encryptedJSON, encryptedKey := t.generateEncryptedRequest(`{id:"abc123"}`)

	encryptedWorkspaceData := crypto.EncryptedPayload{
		EncryptedDataKey: string(encryptedKey),
		EncryptedRequest: base64.StdEncoding.EncodeToString(encryptedJSON),
	}

	encryptedWorkspaceDataMarshalled, err := json.Marshal(&encryptedWorkspaceData)
	t.NoError(err)

	var decryptedData []byte
	optsFunc := messages.WithDecryptedJobAssignedHandler(launcherPrivateKey, func(d []byte) {
		decryptedData = d
	})
	optsFunc(&t.opts)
	payload := messages.ReceivedMessage{
		MessageType:    messages.MessageTypeJobAssigned,
		MessagePayload: json.RawMessage(encryptedWorkspaceDataMarshalled),
	}

	t.opts.MessageHandlers[messages.MessageTypeJobAssigned](t.messageHandler, "", &payload)

	t.ErrorContains(t.payloadError, "could not decode base64 key")
	t.Nil(decryptedData)
}

func (t *MessageHandlerTests) TestWithDecryptedJobAssignedHandler_RequestNotEncodedCorrectly_CallsErrorHandler() {
	launcherPrivateKey, encryptedJSON, encryptedKey := t.generateEncryptedRequest(`{id:"abc123"}`)

	encryptedWorkspaceData := crypto.EncryptedPayload{
		EncryptedDataKey: base64.StdEncoding.EncodeToString(encryptedKey),
		EncryptedRequest: string(encryptedJSON),
	}

	encryptedWorkspaceDataMarshalled, err := json.Marshal(&encryptedWorkspaceData)
	t.NoError(err)

	var decryptedData []byte
	optsFunc := messages.WithDecryptedJobAssignedHandler(launcherPrivateKey, func(d []byte) {
		decryptedData = d
	})
	optsFunc(&t.opts)
	payload := messages.ReceivedMessage{
		MessageType:    messages.MessageTypeJobAssigned,
		MessagePayload: json.RawMessage(encryptedWorkspaceDataMarshalled),
	}

	t.opts.MessageHandlers[messages.MessageTypeJobAssigned](t.messageHandler, "", &payload)

	t.ErrorContains(t.payloadError, "could not decode base64 workspace data")
	t.Nil(decryptedData)
}

func (t *MessageHandlerTests) TestWithDecryptedJobAssignedHandler_PrivateKeyInvalid_CallsErrorHandler() {
	_, encryptedJSON, encryptedKey := t.generateEncryptedRequest(`{id:"abc123"}`)

	encryptedWorkspaceData := crypto.EncryptedPayload{
		EncryptedDataKey: base64.StdEncoding.EncodeToString(encryptedKey),
		EncryptedRequest: base64.StdEncoding.EncodeToString(encryptedJSON),
	}

	encryptedWorkspaceDataMarshalled, err := json.Marshal(&encryptedWorkspaceData)
	t.NoError(err)

	var decryptedData []byte
	optsFunc := messages.WithDecryptedJobAssignedHandler([]byte("Fake key!!!"), func(d []byte) {
		decryptedData = d
	})
	optsFunc(&t.opts)
	payload := messages.ReceivedMessage{
		MessageType:    messages.MessageTypeJobAssigned,
		MessagePayload: json.RawMessage(encryptedWorkspaceDataMarshalled),
	}

	t.opts.MessageHandlers[messages.MessageTypeJobAssigned](t.messageHandler, "", &payload)

	t.ErrorContains(t.payloadError, "could not decode encrypted key")
	t.Nil(decryptedData)
}

func (t *MessageHandlerTests) TestWithDecryptedJobAssignedHandler_EncryptedRequestCorrupted_CallsErrorHandler() {
	launcherPrivateKey, _, encryptedKey := t.generateEncryptedRequest(`{id:"abc123"}`)

	encryptedWorkspaceData := crypto.EncryptedPayload{
		EncryptedDataKey: base64.StdEncoding.EncodeToString(encryptedKey),
		EncryptedRequest: base64.StdEncoding.EncodeToString([]byte("Fake encrypted payload")),
	}

	encryptedWorkspaceDataMarshalled, err := json.Marshal(&encryptedWorkspaceData)
	t.NoError(err)

	var decryptedData []byte
	optsFunc := messages.WithDecryptedJobAssignedHandler(launcherPrivateKey, func(d []byte) {
		decryptedData = d
	})
	optsFunc(&t.opts)
	payload := messages.ReceivedMessage{
		MessageType:    messages.MessageTypeJobAssigned,
		MessagePayload: json.RawMessage(encryptedWorkspaceDataMarshalled),
	}

	t.opts.MessageHandlers[messages.MessageTypeJobAssigned](t.messageHandler, "", &payload)

	t.ErrorContains(t.payloadError, "could not decode encrypted workspace data")
	t.Nil(decryptedData)
}

func (t *MessageHandlerTests) TestWithDecryptedJobAssignedHandler_PayloadInvalid_CallsErrorHandler() {
	var decryptedData []byte
	optsFunc := messages.WithDecryptedJobAssignedHandler(nil, func(d []byte) {
		decryptedData = d
	})
	optsFunc(&t.opts)
	payload := messages.ReceivedMessage{
		MessageType:    messages.MessageTypeJobAssigned,
		MessagePayload: json.RawMessage(invalidJSONPayload),
	}

	t.opts.MessageHandlers[messages.MessageTypeJobAssigned](t.messageHandler, "", &payload)

	t.ErrorContains(t.payloadError, fmt.Sprintf("invalid job_assigned message payload '%s'", invalidJSONPayload))
	t.Nil(decryptedData)
}

func (t *MessageHandlerTests) TestWithScanAssignedHandler_SetsHandler() {
	optsFunc := messages.WithScanAssignedHandler([]byte("abc123"), func(data []byte) {})

	optsFunc(&t.opts)

	t.NotNil(t.opts.MessageHandlers[messages.MessageTypeScanAssigned])
}

func (t *MessageHandlerTests) TestWithScanAssignedHandler_RegisteredTwice_Panics() {
	optsFunc := messages.WithScanAssignedHandler([]byte("abc123"), func(data []byte) {})

	optsFunc(&t.opts)
	t.Panics(func() { optsFunc(&t.opts) })
}

func (t *MessageHandlerTests) TestWithScanAssignedHandler_PayloadValid_DecryptsPayload() {
	launcherPrivateKey, encryptedJSON, encryptedKey := t.generateEncryptedRequest(`{id:"abc123"}`)

	encryptedWorkspaceData := crypto.EncryptedPayload{
		EncryptedDataKey: base64.StdEncoding.EncodeToString(encryptedKey),
		EncryptedRequest: base64.StdEncoding.EncodeToString(encryptedJSON),
	}

	encryptedWorkspaceDataMarshalled, err := json.Marshal(&encryptedWorkspaceData)
	t.NoError(err)

	var decryptedData []byte
	optsFunc := messages.WithScanAssignedHandler(launcherPrivateKey, func(d []byte) {
		decryptedData = d
	})
	optsFunc(&t.opts)
	payload := messages.ReceivedMessage{
		MessageType:    messages.MessageTypeScanAssigned,
		MessagePayload: json.RawMessage(encryptedWorkspaceDataMarshalled),
	}

	t.opts.MessageHandlers[messages.MessageTypeScanAssigned](t.messageHandler, "", &payload)

	t.NoError(t.payloadError)
	t.Equal(`{id:"abc123"}`, string(decryptedData))
}

func (t *MessageHandlerTests) TestWithScanAssignedHandler_PayloadInvalid_CallsErrorHandler() {
	var decryptedData []byte
	optsFunc := messages.WithScanAssignedHandler(nil, func(d []byte) {
		decryptedData = d
	})
	optsFunc(&t.opts)
	payload := messages.ReceivedMessage{
		MessageType:    messages.MessageTypeScanAssigned,
		MessagePayload: json.RawMessage(invalidJSONPayload),
	}

	t.opts.MessageHandlers[messages.MessageTypeScanAssigned](t.messageHandler, "", &payload)

	t.ErrorContains(t.payloadError, fmt.Sprintf("invalid scan_assigned message payload '%s'", invalidJSONPayload))
	t.Nil(decryptedData)
}

func (t *MessageHandlerTests) TestWithScanAssignedHandler_DecryptionFails_CallsErrorHandler() {
	_, encryptedJSON, encryptedKey := t.generateEncryptedRequest(`{id:"abc123"}`)

	encryptedWorkspaceData := crypto.EncryptedPayload{
		EncryptedDataKey: base64.StdEncoding.EncodeToString(encryptedKey),
		EncryptedRequest: base64.StdEncoding.EncodeToString(encryptedJSON),
	}

	encryptedWorkspaceDataMarshalled, err := json.Marshal(&encryptedWorkspaceData)
	t.NoError(err)

	var decryptedData []byte
	optsFunc := messages.WithScanAssignedHandler([]byte("Fake key!!!"), func(d []byte) {
		decryptedData = d
	})
	optsFunc(&t.opts)
	payload := messages.ReceivedMessage{
		MessageType:    messages.MessageTypeScanAssigned,
		MessagePayload: json.RawMessage(encryptedWorkspaceDataMarshalled),
	}

	t.opts.MessageHandlers[messages.MessageTypeScanAssigned](t.messageHandler, "", &payload)

	t.ErrorContains(t.payloadError, "could not decode encrypted key")
	t.Nil(decryptedData)
}

func (t *MessageHandlerTests) TestWithKillHandler_SetsHandler() {
	optsFunc := messages.WithKillHandler(func(_ string, data *messages.KillMessagePayload) {})

	optsFunc(&t.opts)

	t.NotNil(t.opts.MessageHandlers[messages.MessageTypeKill])
}

func (t *MessageHandlerTests) TestWithKillHandler_RegisteredTwice_Panics() {
	optsFunc := messages.WithKillHandler(func(_ string, data *messages.KillMessagePayload) {})

	optsFunc(&t.opts)
	t.Panics(func() { optsFunc(&t.opts) })
}

func (t *MessageHandlerTests) TestWithKillHandler_PayloadValid_UnmarshalsIotPayload() {
	var killMessage *messages.KillMessagePayload
	optsFunc := messages.WithKillHandler(func(_ string, d *messages.KillMessagePayload) {
		killMessage = d
	})
	optsFunc(&t.opts)
	payload := messages.ReceivedMessage{
		MessageType:    messages.MessageTypeKill,
		MessagePayload: json.RawMessage(`{"note":"This worker needs to die!"}`),
	}

	t.opts.MessageHandlers[messages.MessageTypeKill](t.messageHandler, "", &payload)

	t.NoError(t.payloadError)
	t.NotNil(killMessage)
	t.Equal("This worker needs to die!", killMessage.Note)
}

func (t *MessageHandlerTests) TestWithKillHandler_PayloadInvalid_CallsErrorHandler() {
	var killMessage *messages.KillMessagePayload
	optsFunc := messages.WithKillHandler(func(_ string, d *messages.KillMessagePayload) {
		killMessage = d
	})
	optsFunc(&t.opts)
	payload := messages.ReceivedMessage{
		MessageType:    messages.MessageTypeKill,
		MessagePayload: json.RawMessage(invalidJSONPayload),
	}

	t.opts.MessageHandlers[messages.MessageTypeKill](t.messageHandler, "", &payload)

	t.Nil(killMessage)
	t.ErrorContains(t.payloadError, fmt.Sprintf("invalid kill message payload '%s'", invalidJSONPayload))
}

func (t *MessageHandlerTests) TestWithStopHandler_SetsHandler() {
	optsFunc := messages.WithStopHandler(func(data json.RawMessage) {})

	optsFunc(&t.opts)

	t.NotNil(t.opts.MessageHandlers[messages.MessageTypeStop])
}

func (t *MessageHandlerTests) TestWithStopHandler_RegisteredTwice_Panics() {
	optsFunc := messages.WithStopHandler(func(data json.RawMessage) {})

	optsFunc(&t.opts)
	t.Panics(func() { optsFunc(&t.opts) })
}

func (t *MessageHandlerTests) TestWithStopHandler_PayloadValid_UnmarshalsIotPayload() {
	var stopMessage json.RawMessage
	optsFunc := messages.WithStopHandler(func(d json.RawMessage) {
		stopMessage = d
	})
	optsFunc(&t.opts)
	payload := messages.ReceivedMessage{
		MessageType:    messages.MessageTypeStop,
		MessagePayload: json.RawMessage("Stop please!"),
	}

	t.opts.MessageHandlers[messages.MessageTypeStop](t.messageHandler, "", &payload)

	t.NoError(t.payloadError)
	t.NotNil(stopMessage)
	t.Equal("Stop please!", string(stopMessage))
}

func (t *MessageHandlerTests) TestHandlePayload_MessageTypeRegistered_UsesHandler() {
	jobAssignedCalled := false
	stopCalled := false

	t.messageHandler = messages.NewMessageHandler(func(err error) {},
		messages.WithJobAssignedHandler(func(workerULID string, data *crypto.EncryptedPayload) {
			jobAssignedCalled = true
		}),
		messages.WithStopHandler(func(payload json.RawMessage) {
			stopCalled = true
		}))

	jobAssignedPayload, err := json.Marshal(messages.ReceivedMessage{MessageType: messages.MessageTypeJobAssigned})
	t.NoError(err)
	t.messageHandler.HandlePayload("workerID", jobAssignedPayload)

	stopPayload, err := json.Marshal(messages.ReceivedMessage{MessageType: messages.MessageTypeStop})
	t.NoError(err)
	t.messageHandler.HandlePayload("workerID", stopPayload)

	t.True(jobAssignedCalled)
	t.True(stopCalled)
}

func (t *MessageHandlerTests) TestHandlePayload_MessageTypeUnknown_Reports() {
	var reportedType *messages.MessageType

	t.messageHandler = messages.NewMessageHandler(func(err error) {},
		messages.WithJobAssignedHandler(func(workerULID string, data *crypto.EncryptedPayload) {}),
		messages.WithStopHandler(func(payload json.RawMessage) {}),
		messages.WithUnknownMessageTypeHandler(func(t messages.MessageType) {
			reportedType = &t
		}))

	jobAssignedPayload, err := json.Marshal(messages.ReceivedMessage{MessageType: "unknown"})
	t.NoError(err)
	t.messageHandler.HandlePayload("workerID", jobAssignedPayload)

	t.Require().NotNil(reportedType)
	t.Equal("unknown", string(*reportedType))
}

func (t *MessageHandlerTests) TestHandlePayload_DispatchesWithGivenWorkerULID() {
	var reportedError error
	var reportedWorkerULID string

	t.messageHandler = messages.NewMessageHandler(
		func(err error) {
			reportedError = err
		},
		messages.WithJobAssignedHandler(func(workerULID string, data *crypto.EncryptedPayload) {
			reportedWorkerULID = workerULID
		}),
	)

	jobAssignedPayload, err := json.Marshal(messages.ReceivedMessage{MessageType: messages.MessageTypeJobAssigned})
	t.Require().NoError(err)

	t.messageHandler.HandlePayload("workerID", jobAssignedPayload)

	t.Require().NoError(reportedError)
	t.Equal("workerID", reportedWorkerULID)
}

func (t *MessageHandlerTests) TestHandlePayload_InvalidPayload_ReportsError() {
	var reportedError error
	t.messageHandler = messages.NewMessageHandler(func(err error) {
		reportedError = err
	})

	t.messageHandler.HandlePayload("workerID", invalidJSONPayload)

	t.ErrorContains(reportedError, "invalid push event payload")
}

func (t *MessageHandlerTests) TestWithUnknownMessageTypeHandler_RegisteredTwice_Panics() {
	optsFunc := messages.WithUnknownMessageTypeHandler(func(t messages.MessageType) {})

	optsFunc(&t.opts)
	t.Panics(func() { optsFunc(&t.opts) })
}

func TestMessageHandlers(t *testing.T) {
	suite.Run(t, new(MessageHandlerTests))
}
