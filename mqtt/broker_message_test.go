package mqtt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleMessage_PassesWorkerULIDFromTopic(t *testing.T) {
	handler := &recordingPayloadHandler{}
	broker := &MQTTBroker{messageHandler: handler, logger: &recordingLogger{}}

	broker.handleMessage(nil, &mqttMessageMock{payload: []byte(`{}`), topic: "foo/bar/poolID/workerID"})

	require.Len(t, handler.calls, 1)
	assert.Equal(t, "workerID", handler.calls[0].workerULID)
	assert.Equal(t, []byte(`{}`), handler.calls[0].payload)
}

func TestHandleMessage_InvalidTopic_LogsError(t *testing.T) {
	handler := &recordingPayloadHandler{}
	logger := &recordingLogger{}
	broker := &MQTTBroker{messageHandler: handler, logger: logger}

	broker.handleMessage(nil, &mqttMessageMock{payload: []byte(`{}`), topic: "foobar"})

	assert.Empty(t, handler.calls)
	require.Len(t, logger.errors, 1)
	assert.ErrorContains(t, logger.errors[0], "unable to parse topic: foobar")
}

type payloadCall struct {
	workerULID string
	payload    []byte
}

type recordingPayloadHandler struct {
	calls []payloadCall
}

func (h *recordingPayloadHandler) HandlePayload(workerULID string, payload []byte) {
	h.calls = append(h.calls, payloadCall{workerULID: workerULID, payload: payload})
}

type recordingLogger struct {
	errors []error
}

func (l *recordingLogger) Debug(string) {}
func (l *recordingLogger) Info(string)  {}
func (l *recordingLogger) Warn(string)  {}
func (l *recordingLogger) Error(err error, _ string) {
	l.errors = append(l.errors, err)
}

type mqttMessageMock struct {
	payload []byte
	topic   string
}

func (m *mqttMessageMock) Duplicate() bool   { return false }
func (m *mqttMessageMock) Qos() byte         { return 0 }
func (m *mqttMessageMock) Retained() bool    { return false }
func (m *mqttMessageMock) Topic() string     { return m.topic }
func (m *mqttMessageMock) MessageID() uint16 { return 0 }
func (m *mqttMessageMock) Payload() []byte   { return m.payload }
func (m *mqttMessageMock) Ack()              {}
