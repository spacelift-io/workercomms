package workerapi

import "encoding/json"

// ScanEventType is the kind of event reported while running a scan. The
// numbers go over the wire, so only add new types at the end.
type ScanEventType int32

const (
	ScanEventTypeUnknown ScanEventType = iota
	ScanEventTypeFailed
	ScanEventTypeResourcesDiscovered
	ScanEventTypeFinished
	ScanEventTypeStopped
	ScanEventTypeStarted
)

//exhaustive:enforce
var ScanEventTypeNames = map[ScanEventType]string{
	ScanEventTypeUnknown:             "UNKNOWN",
	ScanEventTypeFailed:              "FAILED",
	ScanEventTypeResourcesDiscovered: "RESOURCES_DISCOVERED",
	ScanEventTypeFinished:            "FINISHED",
	ScanEventTypeStopped:             "STOPPED",
	ScanEventTypeStarted:             "STARTED",
}

func (t ScanEventType) String() string {
	return ScanEventTypeNames[t]
}

// ScanEventMessage is one event reported during a scan. Type decides
// how to read Payload, which is empty for events that carry no data.
type ScanEventMessage struct {
	Type    ScanEventType   `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// ScanFailedPayload is the payload for ScanEventTypeFailed.
type ScanFailedPayload struct {
	Reason string          `json:"reason"`
	Code   ScanFailureCode `json:"code,omitempty"`
}
