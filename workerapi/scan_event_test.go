package workerapi_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/spacelift-io/workercomms/workerapi"
)

func TestScanEventType_EnumValues(t *testing.T) {
	assert.Len(t, workerapi.ScanEventTypeNames, 6, "Make sure to update this test if you add a new event types")
	cases := map[workerapi.ScanEventType]int{
		workerapi.ScanEventTypeUnknown:             0,
		workerapi.ScanEventTypeFailed:              1,
		workerapi.ScanEventTypeResourcesDiscovered: 2,
		workerapi.ScanEventTypeFinished:            3,
		workerapi.ScanEventTypeStopped:             4,
		workerapi.ScanEventTypeStarted:             5,
	}
	for kind, want := range cases {
		assert.EqualValues(t, want, kind)
	}
}
